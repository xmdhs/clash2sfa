package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"log/slog"

	"github.com/tidwall/gjson"
	"github.com/xmdhs/clash2singbox/convert"
	"github.com/xmdhs/clash2singbox/httputils"
)

func convert2sing(cxt context.Context, client *http.Client, config,
	sub string, include, exclude string, addTag bool, l *slog.Logger, urlTestOut bool) (map[string]any, []string, error) {
	c, singList, tags, err := httputils.GetAny(cxt, client, sub, addTag)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}

	nodes, err := getExtTag(config)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}
	outs := make([]any, 0, len(nodes)+len(singList))
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
	nb, err := convert.PatchMap([]byte(config), s, include, exclude, outs, extTag, urlTestOut)
	if err != nil {
		return nil, nil, fmt.Errorf("convert2sing: %w", err)
	}
	nodeTag := make([]string, 0, len(s)+len(extTag))

	for _, v := range s {
		if v.Ignored {
			continue
		}
		nodeTag = append(nodeTag, v.Tag)
	}
	nodeTag = append(nodeTag, extTag...)

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
	node     any
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
		nodes = append(nodes, extTag{
			tag:      tag,
			node:     v.Value(),
			nodeType: atype,
		})
	}
	return nodes, nil
}
