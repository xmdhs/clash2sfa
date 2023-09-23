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

func convert2sing(cxt context.Context, client *http.Client, config, sub string, include, exclude string, addTag bool, l *slog.Logger) ([]byte, error) {
	c, err := httputils.GetClash(cxt, client, sub, addTag)
	if err != nil {
		return nil, fmt.Errorf("convert2sing: %w", err)
	}

	nodes, err := getExtTag(config)
	if err != nil {
		return nil, fmt.Errorf("convert2sing: %w", err)
	}
	outs := make([]any, 0, len(nodes))
	extTag := make([]string, 0, len(nodes))

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
	nb, err := convert.Patch([]byte(config), s, include, exclude, outs, extTag...)
	if err != nil {
		return nil, fmt.Errorf("convert2sing: %w", err)
	}
	return nb, nil

}

var ErrFormat = errors.New("错误的格式")

var notNeedType = map[string]struct{}{
	"direct": {},
	"block":  {},
	"dns":    {},
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
		if _, ok := notNeedType[atype]; ok {
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
