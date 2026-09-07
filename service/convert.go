package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2singbox/convert"
	"github.com/xmdhs/clash2singbox/httputils"
	"github.com/xmdhs/clash2singbox/model/singbox"
)

var ErrFormat = errors.New("错误的格式")

// convert 抓取订阅、转换节点并合并进模板。返回打好补丁的配置，以及所有可被分组引用的节点 tag（含可见性），
// 供 configUrlTestParser 展开模板分组里的 include: / exclude: 指令。
func (c *Convert) convert(ctx context.Context, arg model.ConvertArg) (map[string]any, []TagWithVisible, error) {
	clashCfg, singNodes, singTags, err := httputils.GetAny(ctx, c.c, arg.Sub, arg.AddTag)
	if err != nil {
		return nil, nil, fmt.Errorf("convert: %w", err)
	}
	config, err := decodeConfig(arg.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("convert: %w", err)
	}
	tplOuts, err := templateOutbounds(config)
	if err != nil {
		return nil, nil, fmt.Errorf("convert: %w", err)
	}

	// 外部节点 = 模板自带的 outbound + 订阅直接给出的 sing-box outbound；分组本身不算节点
	outs := make([]map[string]any, 0, len(tplOuts)+len(singNodes))
	extTags := make([]string, 0, len(tplOuts)+len(singTags))
	for _, o := range tplOuts {
		outs = append(outs, o.node)
		if !o.isGroup() {
			extTags = append(extTags, o.tag)
		}
	}
	outs = append(outs, singNodes...)
	extTags = append(extTags, singTags...)

	s, eps, err := convert.Clash2sing(clashCfg, arg.Ver)
	if err != nil {
		c.l.DebugContext(ctx, err.Error()) // 个别节点转换失败只记日志，不影响其余节点
	}
	s, outs, tags := expandDetours(s, eps, config, outs, extTags)

	extOut := make([]any, len(outs))
	for i, o := range outs {
		extOut[i] = o
	}
	config, err = convert.PatchMapFromMap(config, s, eps, arg.Include, arg.Exclude, extOut, extTags, !arg.DisableUrlTest, arg.OutFields)
	if err != nil {
		return nil, nil, fmt.Errorf("convert: %w", err)
	}
	return config, nodeTags(s, eps, tags), nil
}

// decodeConfig 解码 JSON 模板；非法 JSON 或不是对象时返回 ErrFormat。
func decodeConfig(config []byte) (map[string]any, error) {
	var d map[string]any
	if err := json.Unmarshal(config, &d); err != nil {
		return nil, fmt.Errorf("decodeConfig: %w: %v", ErrFormat, err)
	}
	if d == nil {
		return nil, fmt.Errorf("decodeConfig: %w: 配置必须是 JSON 对象", ErrFormat)
	}
	return d, nil
}

// templateOutbound 是模板里用户自定义的 outbound（节点或分组）。
type templateOutbound struct {
	tag      string
	node     map[string]any
	nodeType string
}

func (o templateOutbound) isGroup() bool {
	return o.nodeType == "urltest" || o.nodeType == "selector"
}

// builtinTags 是 PatchMapFromMap 会自动补上的 outbound，模板里的同名项不作为用户节点处理。
var builtinTags = map[string]bool{"direct": true, "block": true, "dns-out": true}

// templateOutbounds 提取模板 outbounds 中除 direct / block / dns-out 之外的全部 outbound。
func templateOutbounds(config map[string]any) ([]templateOutbound, error) {
	raw, ok := config["outbounds"]
	if !ok {
		return nil, fmt.Errorf("templateOutbounds: 缺少 outbounds: %w", ErrFormat)
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("templateOutbounds: outbounds 必须是数组: %w", ErrFormat)
	}
	outs := make([]templateOutbound, 0, len(list))
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := m["tag"].(string)
		if builtinTags[tag] {
			continue
		}
		typ, _ := m["type"].(string)
		outs = append(outs, templateOutbound{tag: tag, node: m, nodeType: typ})
	}
	return outs, nil
}

// TagWithVisible 是一个可被分组引用的节点 tag。Visible 非空时只在列出的分组中可见，"_hide" 表示对所有分组隐藏。
type TagWithVisible struct {
	Tag     string
	Visible []string
}

func tagsWithVisible(tags []string) []TagWithVisible {
	out := make([]TagWithVisible, 0, len(tags))
	for _, tag := range tags {
		out = append(out, TagWithVisible{Tag: tag})
	}
	return out
}

// nodeTags 汇总所有可被分组引用的节点：转换出的节点（跳过只作 detour 的 Ignored 节点）、endpoint 与外部节点。
func nodeTags(s []singbox.SingBoxOut, eps []*singbox.SingBoxEndpoint, ext []TagWithVisible) []TagWithVisible {
	tags := make([]TagWithVisible, 0, len(s)+len(eps)+len(ext))
	for _, v := range s {
		if !v.Ignored {
			tags = append(tags, TagWithVisible{Tag: v.Tag, Visible: v.Visible})
		}
	}
	for _, ep := range eps {
		if ep != nil && ep.Tag != "" {
			tags = append(tags, TagWithVisible{Tag: ep.Tag})
		}
	}
	return append(tags, ext...)
}

// detourGroup 是模板里设置了 detour 的分组。
type detourGroup struct {
	tag, detour string
}

// groupsWithDetour 返回模板中带 detour 字段的分组（即含 outbounds 字段的 outbound）。
func groupsWithDetour(config map[string]any) []detourGroup {
	list, _ := config["outbounds"].([]any)
	var groups []detourGroup
	for _, item := range list {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if _, isGroup := m["outbounds"]; !isGroup {
			continue
		}
		detour, _ := m["detour"].(string)
		if detour == "" {
			continue
		}
		tag, _ := m["tag"].(string)
		groups = append(groups, detourGroup{tag: tag, detour: detour})
	}
	return groups
}

// expandDetours 处理模板里带 detour 的分组（selector / urltest）：
// 分组的 detour 指向一条出站链（可多级，沿各节点的 detour 字段追踪）。为每个普通节点复制一份该链，
// 链尾指向这个节点、tag 改为 "节点 - 链节点 [分组]"，让分组可以在"先经 detour 出站、再走某节点"的组合中选择。
// 复制出的节点通过 Visible 标记只对该分组可见，链的中间节点标记为 _hide。
func expandDetours(s []singbox.SingBoxOut, eps []*singbox.SingBoxEndpoint, config map[string]any, outs []map[string]any, extTags []string) ([]singbox.SingBoxOut, []map[string]any, []TagWithVisible) {
	tags := tagsWithVisible(extTags)
	groups := groupsWithDetour(config)
	if len(groups) == 0 {
		return s, outs, tags
	}

	allTags := selectableTags(s, eps, outs)
	if len(allTags) == 0 {
		return s, outs, tags
	}

	// 索引表延迟到首次需要时再建：无 detour 分组或无可选节点时直接返回，
	// 避免每次请求都把全量 SingBoxOut（760B/个）复制进堆上 map。
	var singByTag map[string]singbox.SingBoxOut
	var anyByTag map[string]map[string]any

	for _, g := range groups {
		if singByTag == nil {
			singByTag = make(map[string]singbox.SingBoxOut, len(s))
			for _, v := range s {
				singByTag[v.Tag] = v
			}
		}
		if anyByTag == nil {
			anyByTag = make(map[string]map[string]any, len(outs))
			for _, o := range outs {
				tag, _ := o["tag"].(string)
				anyByTag[tag] = o
			}
		}
		singTags, singChain := detourChain(g.detour, singByTag, singTagDetour)
		anyTags, anyChain := detourChain(g.detour, anyByTag, anyTagDetour)
		if len(singChain) == 0 && len(anyChain) == 0 {
			continue // detour 指向不存在的 tag，无需展开
		}
		inChain := make(map[string]bool, len(singTags)+len(anyTags))
		for _, t := range singTags {
			inChain[t] = true
		}
		for _, t := range anyTags {
			inChain[t] = true
		}

		for _, nodeTag := range allTags {
			if inChain[nodeTag] {
				continue // 链上的节点不能再作为自己的出口
			}
			s = append(s, chainSingCopies(singChain, nodeTag, g.tag)...)
			copies, copyTags := chainAnyCopies(anyChain, nodeTag, g.tag)
			outs = append(outs, copies...)
			tags = append(tags, copyTags...)
		}
	}
	return s, outs, tags
}

// selectableTags 返回可作为链尾出口的全部 tag：转换出的节点（跳过 Ignored）、endpoint，以及外部节点（跳过分组）。
func selectableTags(s []singbox.SingBoxOut, eps []*singbox.SingBoxEndpoint, outs []map[string]any) []string {
	tags := make([]string, 0, len(s)+len(eps)+len(outs))
	for _, v := range s {
		if !v.Ignored {
			tags = append(tags, v.Tag)
		}
	}
	for _, ep := range eps {
		if ep != nil && ep.Tag != "" {
			tags = append(tags, ep.Tag)
		}
	}
	seen := make(map[string]bool, len(outs))
	for _, o := range outs {
		typ, _ := o["type"].(string)
		tag, _ := o["tag"].(string)
		if typ == "urltest" || typ == "selector" || seen[tag] {
			continue
		}
		seen[tag] = true
		tags = append(tags, tag)
	}
	return tags
}

func singTagDetour(o singbox.SingBoxOut) (tag, detour string) {
	return o.Tag, o.Detour
}

func anyTagDetour(o map[string]any) (tag, detour string) {
	tag, _ = o["tag"].(string)
	detour, _ = o["detour"].(string)
	return tag, detour
}

// detourChain 从 start 出发沿 detour 字段追踪出站链，返回链上各节点的 tag 与节点本身；遇到未知 tag 或环时停止。
func detourChain[T any](start string, nodes map[string]T, tagAndDetour func(T) (tag, detour string)) ([]string, []T) {
	var tags []string
	var chain []T
	visited := map[string]bool{}
	for cur := start; cur != ""; {
		node, ok := nodes[cur]
		if !ok {
			break
		}
		tag, next := tagAndDetour(node)
		if visited[tag] {
			break
		}
		visited[tag] = true
		tags = append(tags, tag)
		chain = append(chain, node)
		cur = next
	}
	return tags, chain
}

// chainSingCopies 为节点 nodeTag 复制一条由转换节点组成的出站链：链尾指向 nodeTag，链首在分组 group 中可见，中间节点隐藏。
func chainSingCopies(chain []singbox.SingBoxOut, nodeTag, group string) []singbox.SingBoxOut {
	copies := make([]singbox.SingBoxOut, 0, len(chain))
	detour := nodeTag
	for i, c := range slices.Backward(chain) {
		c.Detour = detour
		c.Tag = chainTag(nodeTag, c.Tag, group)
		c.Visible = chainVisible(i, group)
		detour = c.Tag
		copies = append(copies, c)
	}
	return copies
}

// chainAnyCopies 与 chainSingCopies 相同，但处理 map 形式的外部节点，并一并返回副本的 tag 与可见性。
func chainAnyCopies(chain []map[string]any, nodeTag, group string) ([]map[string]any, []TagWithVisible) {
	copies := make([]map[string]any, 0, len(chain))
	tags := make([]TagWithVisible, 0, len(chain))
	detour := nodeTag
	for i, c := range slices.Backward(chain) {
		c := maps.Clone(c)
		origTag, _ := c["tag"].(string)
		newTag := chainTag(nodeTag, origTag, group)
		c["detour"] = detour
		c["tag"] = newTag
		tags = append(tags, TagWithVisible{Tag: newTag, Visible: chainVisible(i, group)})
		detour = newTag
		copies = append(copies, c)
	}
	return copies, tags
}

func chainTag(nodeTag, linkTag, group string) string {
	return nodeTag + " - " + linkTag + " [" + group + "]"
}

// chainVisible 链首（i == 0）只对分组 group 可见，其余中间节点对所有分组隐藏。
func chainVisible(i int, group string) []string {
	if i == 0 {
		return []string{group}
	}
	return []string{"_hide"}
}
