package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"github.com/tidwall/jsonc"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/utils"
	"github.com/xmdhs/clash2singbox/httputils"
)

// ErrJson 保留以兼容旧调用方。
var ErrJson = errors.New("错误的 json")

// maxConfigBytes 是远程模板（configurl）的读取上限。
const maxConfigBytes = 10 * 1000 * 1000

// Convert 负责把订阅转换成 sing-box 配置。
type Convert struct {
	c *http.Client
	l *slog.Logger
}

func NewConvert(c *http.Client, l *slog.Logger) *Convert {
	return &Convert{c: c, l: l}
}

// clientForUA 在 ua 非空时返回覆盖 User-Agent 的 client 副本；为空则返回原 client。
// 上游 httputils 写死了订阅抓取 UA，这里用 Transport 包装覆盖，不改上游签名。
func (c *Convert) clientForUA(ua string) *http.Client {
	if ua == "" {
		return c.c
	}
	base := c.c.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	cp := *c.c
	cp.Transport = userAgentTransport{base: base, ua: ua}
	return &cp
}

type userAgentTransport struct {
	base http.RoundTripper
	ua   string
}

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.Header.Set("User-Agent", t.ua)
	return t.base.RoundTrip(req)
}

// MakeConfig 生成最终的 sing-box 配置。模板优先级：arg.ConfigUrl 指向的远程模板 > arg.Config > configByte（默认模板），
// 模板支持 JSONC。浏览器请求（按 userAgent 判断）返回缩进格式，客户端请求返回紧凑格式。
// arg.UserAgent 非空时，抓取订阅与远程模板改用该 UA，否则沿用上游库默认 UA。
func (c *Convert) MakeConfig(ctx context.Context, arg model.ConvertArg, configByte []byte, userAgent string) ([]byte, error) {
	cc := *c
	cc.c = cc.clientForUA(arg.UserAgent)
	switch {
	case arg.ConfigUrl != "":
		b, err := httputils.HttpGet(ctx, cc.c, arg.ConfigUrl, maxConfigBytes)
		if err != nil {
			return nil, fmt.Errorf("MakeConfig: %w", err)
		}
		arg.Config = b
	case arg.Config == nil:
		arg.Config = configByte
	}
	arg.Config = jsonc.ToJSON(arg.Config)

	config, tags, err := cc.convert(ctx, arg)
	if err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	if _, err := configUrlTestParser(config, tags); err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	out, err := encodeConfig(config, utils.IsBrowser(userAgent))
	if err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	return out, nil
}

// encodeConfig 序列化配置；pretty 为 true 时输出 4 空格缩进。
func encodeConfig(config map[string]any, pretty bool) ([]byte, error) {
	if !pretty {
		return json.Marshal(config)
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "    ")
	if err := enc.Encode(config); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// filterTags 先按 include 保留匹配的 tag，再按 exclude 剔除匹配的 tag；空正则表示不过滤。
func filterTags(tags []string, include, exclude string) ([]string, error) {
	tags, err := filterByRegexp(tags, include, true)
	if err != nil {
		return nil, fmt.Errorf("filterTags: %w", err)
	}
	tags, err = filterByRegexp(tags, exclude, false)
	if err != nil {
		return nil, fmt.Errorf("filterTags: %w", err)
	}
	return tags, nil
}

// filterByRegexp 用正则筛选 tags：keepMatch 为 true 保留匹配项，否则剔除匹配项；reg 为空时原样返回。
func filterByRegexp(tags []string, reg string, keepMatch bool) ([]string, error) {
	if reg == "" {
		return tags, nil
	}
	re, err := regexp.Compile(reg)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	out := make([]string, 0, len(tags))
	for _, tag := range tags {
		if re.MatchString(tag) == keepMatch {
			out = append(out, tag)
		}
	}
	return out, nil
}

// configUrlTestParser 展开模板分组 outbounds 里的 "include: <正则>" / "exclude: <正则>" 指令为具体节点。
// 带 detour 的分组只能看到为它生成的链式副本（Visible 含该分组 tag），其余分组看到所有普通节点。
func configUrlTestParser(config map[string]any, tags []TagWithVisible) (map[string]any, error) {
	outbounds, ok := config["outbounds"].([]any)
	if !ok {
		return nil, fmt.Errorf("configUrlTestParser: outbounds is not []any or missing")
	}
	for _, item := range outbounds {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		list, _ := m["outbounds"].([]any)
		if len(list) == 0 {
			continue
		}
		group, _ := m["tag"].(string)
		detour, _ := m["detour"].(string)
		visibleTo := ""
		if group != "" && detour != "" {
			visibleTo = group
			delete(m, "detour") // detour 已通过链式副本体现，分组本身不再需要
		}
		expanded, err := urlTestParser(stringItems(list), visibleTags(tags, visibleTo))
		if err != nil {
			return nil, fmt.Errorf("configUrlTestParser: %w", err)
		}
		if expanded != nil {
			m["outbounds"] = expanded
		}
	}
	return config, nil
}

// visibleTags 返回分组 group 可见的节点 tag；group 为空时返回所有无可见性限制的普通节点。
func visibleTags(tags []TagWithVisible, group string) []string {
	out := make([]string, 0, len(tags))
	for _, t := range tags {
		visible := len(t.Visible) == 0
		if group != "" {
			visible = slices.Contains(t.Visible, group)
		}
		if visible {
			out = append(out, t.Tag)
		}
	}
	return out
}

func stringItems(list []any) []string {
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// urlTestParser 处理分组的 outbounds 列表：把 "include: <正则>" / "exclude: <正则>" 指令替换为从 tags 中筛出的节点，
// 其余静态项保留在前并去重。没有指令时返回 nil 表示无需改动。
func urlTestParser(outbounds, tags []string) ([]string, error) {
	var include, exclude string
	static := make([]string, 0, len(outbounds))
	for _, s := range outbounds {
		if v, ok := strings.CutPrefix(s, "include: "); ok {
			include = v
		} else if v, ok := strings.CutPrefix(s, "exclude: "); ok {
			exclude = v
		} else {
			static = append(static, s)
		}
	}
	if include == "" && exclude == "" {
		return nil, nil
	}
	matched, err := filterTags(tags, include, exclude)
	if err != nil {
		return nil, fmt.Errorf("urlTestParser: %w", err)
	}
	return uniq(append(static, matched...)), nil
}

// uniq 去重并保持首次出现的顺序。
func uniq(items []string) []string {
	seen := make(map[string]bool, len(items))
	out := make([]string, 0, len(items))
	for _, item := range items {
		if !seen[item] {
			seen[item] = true
			out = append(out, item)
		}
	}
	return out
}
