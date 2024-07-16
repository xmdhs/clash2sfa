package service

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"

	"log/slog"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/xmdhs/clash2sfa/utils"
	"github.com/xmdhs/clash2singbox/convert"
	"github.com/xmdhs/clash2singbox/httputils"
	"github.com/xmdhs/clash2singbox/model/singbox"
)

func convert2sing(cxt context.Context, client *http.Client, config,
	sub string, include, exclude string, addTag bool, l *slog.Logger, urlTestOut bool) (map[string]any, []TagWithVisible, error) {
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

	s, err := convert.Clash2sing(c)
	if err != nil {
		l.DebugContext(cxt, err.Error())
	}
	outs = append(outs, singList...)
	extTag = append(extTag, tags...)

	s, outs, extTagWithV := urlTestDetourSet(s, config, outs, extTag)

	nb, err := convert.PatchMap([]byte(config), s, include, exclude, lo.Map(outs, func(item map[string]any, index int) any {
		return item
	}), extTag, urlTestOut)
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

func getExtTag(config string) ([]extTag, error) {
	vaild := gjson.Valid(config)
	if !vaild {
		return nil, fmt.Errorf("getExtTag: %w", ErrFormat)
	}

	outs := gjson.Get(config, "outbounds")
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

func urlTestDetourSet(s []singbox.SingBoxOut, config string, outs []map[string]any, extTag []string) ([]singbox.SingBoxOut, []map[string]any, []TagWithVisible) {
	j := gjson.Parse(config)
	newSingOut := make([]singbox.SingBoxOut, 0)
	newAnyOut := make([]map[string]any, 0)
	newExtTag := make([]TagWithVisible, 0)

	list := j.Get("outbounds.#(outbounds)#").Array()

	notAdd := map[string]struct{}{}
	for _, v := range outs {
		d := utils.AnyGet[string](v, "detour")
		if d != "" {
			notAdd[d] = struct{}{}
		}
	}

	update := false

	for _, value := range list {
		detour := value.Get("detour").String()
		tag := value.Get("tag").String()
		if detour != "" {
			update = true
			for _, v := range s {
				if v.Ignored || v.Tag == detour {
					continue
				}
				v.Tag = fmt.Sprintf("%v - %v [%v]", v.Tag, detour, tag)
				v.Detour = detour
				v.Visible = append(v.Visible, tag)
				newSingOut = append(newSingOut, v)
			}
			for _, v := range outs {
				newAnyOut = append(newAnyOut, maps.Clone(v))

				t := utils.AnyGet[string](v, "type")
				if t == "urltest" || t == "selector" {
					continue
				}

				oldTag := utils.AnyGet[string](v, "tag")
				if _, ok := notAdd[oldTag]; ok {
					continue
				}

				newTag := fmt.Sprintf("%v - %v [%v]", oldTag, detour, tag)
				utils.AnySet(&v, newTag, "tag")
				utils.AnySet(&v, detour, "detour")
				newAnyOut = append(newAnyOut, v)
				newExtTag = append(newExtTag, TagWithVisible{
					Tag:     newTag,
					Visible: []string{tag},
				})
			}
		}
	}

	tagV := lo.Map(extTag, func(item string, index int) TagWithVisible {
		return TagWithVisible{
			Tag: item,
		}
	})

	if update {
		tagV = append(tagV, newExtTag...)
		return append(s, newSingOut...), newAnyOut, append(tagV, newExtTag...)
	}

	return s, outs, tagV
}
