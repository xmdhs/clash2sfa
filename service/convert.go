package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"sync"
	"sync/atomic"

	"log/slog"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/xmdhs/clash2sfa/utils"
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

	nodes, err := getExtTag(config)
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

	s, err := convert.Clash2sing(c, ver)
	if err != nil {
		l.DebugContext(cxt, err.Error())
	}
	outs = append(outs, singList...)
	extTag = append(extTag, tags...)

	s, outs, extTagWithV := urlTestDetourSet(s, config, outs, extTag)

	nb, err := convert.PatchMap([]byte(config), s, include, exclude, lo.Map(outs, func(item map[string]any, index int) any {
		return item
	}), extTag, urlTestOut, outFields)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}
	nodeTag := make([]TagWithVisible, 0, len(s)+len(extTagWithV))

	for _, v := range s {
		if v.Ignored {
			continue
		}
		nodeTag = append(nodeTag, TagWithVisible{
			Tag:     v.Tag,
			Visible: v.Visible,
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

func getExtTag(config []byte) ([]extTag, error) {
	vaild := gjson.ValidBytes(config)
	if !vaild {
		return nil, fmt.Errorf("getExtTag: %w", ErrFormat)
	}

	outs := gjson.GetBytes(config, "outbounds")
	if !outs.Exists() {
		return nil, fmt.Errorf("getExtTag: %w", ErrFormat)
	}
	nodes := []extTag{}
	for _, v := range outs.Array() {
		tag := v.Get("tag").String()
		atype := v.Get("type").String()
		if _, ok := notNeedTag[tag]; ok {
			continue
		}
		m, ok := v.Value().(map[string]any)
		if ok {
			nodes = append(nodes, extTag{
				tag:      tag,
				node:     m,
				nodeType: atype,
			})
		}
	}
	return nodes, nil
}

type TagWithVisible struct {
	Tag     string
	Visible []string
}

func urlTestDetourSet(s []singbox.SingBoxOut, config []byte, outs []map[string]any, extTag []string) ([]singbox.SingBoxOut, []map[string]any, []TagWithVisible) {
	j := gjson.ParseBytes(config)
	newSingOut := make([]singbox.SingBoxOut, 0)
	newAnyOut := make([]map[string]any, 0)
	newExtTag := make([]TagWithVisible, 0)

	list := j.Get("outbounds.#(outbounds)#").Array()

	update := atomic.Bool{}

	type OnceValue struct {
		singMap map[string]singbox.SingBoxOut
		anyMap  map[string]map[string]any
		allTags []string
	}

	mapF := sync.OnceValue(func() OnceValue {
		singMap := lo.SliceToMap(s, func(item singbox.SingBoxOut) (string, singbox.SingBoxOut) {
			return item.Tag, item
		})
		anyMap := lo.SliceToMap(outs, func(item map[string]any) (string, map[string]any) {
			return utils.AnyGet[string](item, "tag"), item
		})
		allTags := make([]string, 0, len(s)+len(outs))
		for _, v := range s {
			if v.Ignored {
				continue
			}
			allTags = append(allTags, v.Tag)
		}
		for k, v := range anyMap {
			t := utils.AnyGet[string](v, "type")
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
		detour := value.Get("detour").String()
		tag := value.Get("tag").String()
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
				for i := len(singDList) - 1; i >= 0; i-- {
					singDetour := singDList[i]
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
				for i := len(anyDList) - 1; i >= 0; i-- {
					anyDetour := maps.Clone(anyDList[i])
					if prevTag == "" {
						utils.AnySet(&anyDetour, nowTag, "detour")
					} else {
						utils.AnySet(&anyDetour, prevTag, "detour")
					}
					prevTag = fmt.Sprintf("%v - %v [%v]", nowTag, utils.AnyGet[string](anyDetour, "tag"), tag)
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
					utils.AnySet(&anyDetour, prevTag, "tag")
					newAnyOut = append(newAnyOut, anyDetour)
				}
			}
		}
	}

	tagV := lo.Map(extTag, func(item string, index int) TagWithVisible {
		return TagWithVisible{
			Tag: item,
		}
	})

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
		tag := utils.AnyGet[string](a, "tag")
		// 检查循环引用
		if visited[tag] {
			break
		}
		visited[tag] = true
		tags = append(tags, tag)
		anyOut = append(anyOut, a)
		detour = utils.AnyGet[string](a, "detour")
		if detour == "" {
			break
		}
	}
	return tags, anyOut
}
