package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"

	"log/slog"

	"github.com/xmdhs/clash2singbox/convert"
	"github.com/xmdhs/clash2singbox/httputils"
	"github.com/xmdhs/clash2singbox/model"
	"github.com/xmdhs/clash2singbox/model/singbox"
)

func convert2sing(cxt context.Context, client *http.Client, config []byte,
	sub string, include, exclude string, addTag bool, l *slog.Logger, urlTestOut bool, outFields bool, ver model.SingBoxVer) (map[string]any, []TagWithVisible, error) {
	c, singList, tags, err := httputils.GetAny(cxt, client, sub, addTag)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}

	configMap, err := decodeConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}
	nodes, err := getExtTagFromMap(configMap)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}
	outs := make([]map[string]any, 0, len(nodes)+len(singList))
	extTag := make([]string, 0, len(nodes)+len(tags))

	for _, v := range nodes {
		outs = append(outs, v.node)
		if v.nodeType != "urltest" && v.nodeType != "selector" {
			extTag = append(extTag, v.tag)
		}
	}

	s, eps, err := convert.Clash2sing(c, ver)
	if err != nil {
		l.DebugContext(cxt, err.Error())
	}
	outs = append(outs, singList...)
	extTag = append(extTag, tags...)

	s, outs, extTagWithV := urlTestDetourSetFromMap(s, eps, configMap, outs, extTag)

	extOut := make([]any, len(outs))
	for i, item := range outs {
		extOut[i] = item
	}
	nb, err := convert.PatchMapFromMap(configMap, s, eps, include, exclude, extOut, extTag, urlTestOut, outFields)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}
	nodeTag := make([]TagWithVisible, 0, len(s)+len(eps)+len(extTagWithV))

	for _, v := range s {
		if v.Ignored {
			continue
		}
		nodeTag = append(nodeTag, TagWithVisible{
			Tag:     v.Tag,
			Visible: v.Visible,
		})
	}
	for _, ep := range eps {
		if ep == nil || ep.Tag == "" {
			continue
		}
		nodeTag = append(nodeTag, TagWithVisible{
			Tag: ep.Tag,
		})
	}
	nodeTag = append(nodeTag, extTagWithV...)
	return nb, nodeTag, nil
}

var ErrFormat = errors.New("错误的格式")

var notNeedTag = map[string]struct{}{
	"direct":  {},
	"block":   {},
	"dns-out": {},
}

type extTag struct {
	tag      string
	node     map[string]any
	nodeType string
}

func decodeConfig(config []byte) (map[string]any, error) {
	d := map[string]any{}
	if err := json.Unmarshal(config, &d); err != nil {
		return nil, fmt.Errorf("decodeConfig: %w", ErrFormat)
	}
	return d, nil
}

func getExtTag(config []byte) ([]extTag, error) {
	d, err := decodeConfig(config)
	if err != nil {
		return nil, fmt.Errorf("getExtTag: %w", err)
	}
	return getExtTagFromMap(d)
}

func getExtTagFromMap(config map[string]any) ([]extTag, error) {
	rawOutbounds, exists := config["outbounds"]
	if !exists {
		return nil, fmt.Errorf("getExtTag: %w", ErrFormat)
	}

	var outbounds []any
	switch v := rawOutbounds.(type) {
	case []any:
		outbounds = v
	case map[string]any:
		// gjson.Array 将非数组对象视为单个元素；保持原有
		// 行为，将对象作为单个 outbound 处理。
		outbounds = []any{v}
	default:
		outbounds = nil
	}

	nodes := make([]extTag, 0, len(outbounds))
	for _, raw := range outbounds {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		tag, _ := m["tag"].(string)
		atype, _ := m["type"].(string)
		if _, ok := notNeedTag[tag]; ok {
			continue
		}
		nodes = append(nodes, extTag{
			tag:      tag,
			node:     m,
			nodeType: atype,
		})
	}
	return nodes, nil
}

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

func outboundWithLists(config map[string]any) []map[string]any {
	raw, ok := config["outbounds"]
	if !ok {
		return nil
	}
	outbounds, ok := raw.([]any)
	if !ok {
		return nil
	}
	list := make([]map[string]any, 0, len(outbounds))
	for _, raw := range outbounds {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		if _, ok := m["outbounds"]; ok {
			list = append(list, m)
		}
	}
	return list
}

func urlTestDetourSet(s []singbox.SingBoxOut, eps []*singbox.SingBoxEndpoint, config []byte, outs []map[string]any, extTag []string) ([]singbox.SingBoxOut, []map[string]any, []TagWithVisible) {
	d, err := decodeConfig(config)
	if err != nil {
		return s, outs, tagsWithVisible(extTag)
	}
	return urlTestDetourSetFromMap(s, eps, d, outs, extTag)
}

func urlTestDetourSetFromMap(s []singbox.SingBoxOut, eps []*singbox.SingBoxEndpoint, config map[string]any, outs []map[string]any, extTag []string) ([]singbox.SingBoxOut, []map[string]any, []TagWithVisible) {
	newSingOut := make([]singbox.SingBoxOut, 0)
	newAnyOut := make([]map[string]any, 0)
	newExtTag := make([]TagWithVisible, 0)

	list := outboundWithLists(config)

	update := atomic.Bool{}

	type OnceValue struct {
		singMap map[string]singbox.SingBoxOut
		anyMap  map[string]map[string]any
		allTags []string
	}

	mapF := sync.OnceValue(func() OnceValue {
		singMap := make(map[string]singbox.SingBoxOut, len(s))
		for _, item := range s {
			singMap[item.Tag] = item
		}
		anyMap := make(map[string]map[string]any, len(outs))
		for _, item := range outs {
			tag, _ := item["tag"].(string)
			anyMap[tag] = item
		}
		allTags := make([]string, 0, len(s)+len(eps)+len(outs))
		for _, v := range s {
			if v.Ignored {
				continue
			}
			allTags = append(allTags, v.Tag)
		}
		for _, ep := range eps {
			if ep == nil || ep.Tag == "" {
				continue
			}
			allTags = append(allTags, ep.Tag)
		}
		for k, v := range anyMap {
			t, _ := v["type"].(string)
			if t == "urltest" || t == "selector" {
				continue
			}
			allTags = append(allTags, k)
		}

		update.Store(true)

		return OnceValue{
			singMap: singMap,
			anyMap:  anyMap,
			allTags: allTags,
		}
	})

	for _, value := range list {
		detour, _ := value["detour"].(string)
		tag, _ := value["tag"].(string)
		if detour != "" {
			m := mapF()
			notAdd := map[string]struct{}{}

			tags, singDList := singDetourList(detour, m.singMap)
			for _, v := range tags {
				notAdd[v] = struct{}{}
			}
			tags, anyDList := anyDetourList(detour, m.anyMap)
			for _, v := range tags {
				notAdd[v] = struct{}{}
			}

			for _, nowTag := range m.allTags {
				if _, ok := notAdd[nowTag]; ok {
					continue
				}
				prevTag := ""
				for i, singDetour := range slices.Backward(singDList) {

					if prevTag == "" {
						singDetour.Detour = nowTag
					} else {
						singDetour.Detour = prevTag
					}
					if i == 0 {
						singDetour.Visible = []string{tag}
					} else {
						singDetour.Visible = []string{"_hide"}
					}
					prevTag = fmt.Sprintf("%v - %v [%v]", nowTag, singDetour.Tag, tag)
					singDetour.Tag = prevTag
					newSingOut = append(newSingOut, singDetour)
				}
				prevTag = ""
				for i, a := range slices.Backward(anyDList) {
					anyDetour := maps.Clone(a)
					if prevTag == "" {
						anyDetour["detour"] = nowTag
					} else {
						anyDetour["detour"] = prevTag
					}
					anyDetourTag, _ := anyDetour["tag"].(string)
					prevTag = fmt.Sprintf("%v - %v [%v]", nowTag, anyDetourTag, tag)
					if i == 0 {
						newExtTag = append(newExtTag, TagWithVisible{
							Tag:     prevTag,
							Visible: []string{tag},
						})
					} else {
						newExtTag = append(newExtTag, TagWithVisible{
							Tag:     prevTag,
							Visible: []string{"_hide"},
						})
					}
					anyDetour["tag"] = prevTag
					newAnyOut = append(newAnyOut, anyDetour)
				}
			}
		}
	}

	tagV := tagsWithVisible(extTag)

	if update.Load() {
		return append(s, newSingOut...), append(outs, newAnyOut...), append(tagV, newExtTag...)
	}

	return s, outs, tagV
}

func singDetourList(detour string, singMap map[string]singbox.SingBoxOut) ([]string, []singbox.SingBoxOut) {
	tags := []string{}
	singOut := []singbox.SingBoxOut{}
	visited := make(map[string]bool)

	for {
		s, ok := singMap[detour]
		if !ok {
			break
		}
		// 检查循环引用
		if visited[s.Tag] {
			break
		}
		visited[s.Tag] = true
		tags = append(tags, s.Tag)
		singOut = append(singOut, s)
		detour = s.Detour
		if detour == "" {
			break
		}
	}
	return tags, singOut
}

func anyDetourList(detour string, anyMap map[string]map[string]any) ([]string, []map[string]any) {
	tags := []string{}
	anyOut := []map[string]any{}
	visited := make(map[string]bool)

	for {
		a, ok := anyMap[detour]
		if !ok {
			break
		}
		tag, _ := a["tag"].(string)
		// 检查循环引用
		if visited[tag] {
			break
		}
		visited[tag] = true
		tags = append(tags, tag)
		anyOut = append(anyOut, a)
		detour, _ = a["detour"].(string)
		if detour == "" {
			break
		}
	}
	return tags, anyOut
}
