package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"log/slog"

	"github.com/xmdhs/clash2singbox/convert"
	"github.com/xmdhs/clash2singbox/httputils"
)

func convert2sing(cxt context.Context, client *http.Client, config, sub string, include, exclude string, l *slog.Logger) ([]byte, error) {
	c, err := httputils.GetClash(cxt, client, sub)
	if err != nil {
		return nil, fmt.Errorf("convert2sing: %w", err)
	}

	extTag, outs, err := getExtTag(config)
	if err != nil {
		return nil, fmt.Errorf("convert2sing: %w", err)
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

func getExtTag(config string) ([]string, []any, error) {
	singc := map[string]interface{}{}
	err := json.Unmarshal([]byte(config), &singc)
	if err != nil {
		return nil, nil, fmt.Errorf("getExtTag: %w", err)
	}
	outs, ok := singc["outbounds"].([]interface{})
	if !ok {
		return nil, nil, fmt.Errorf("getExtTag: %w", ErrFormat)
	}
	tags := []string{}
	anys := []any{}
	for _, v := range outs {
		mv, ok := v.(map[string]interface{})
		if !ok {
			return nil, nil, fmt.Errorf("getExtTag: %w", ErrFormat)
		}
		atype, ok := mv["type"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("getExtTag: %w", ErrFormat)
		}
		tag, ok := mv["tag"].(string)
		if !ok {
			return nil, nil, fmt.Errorf("getExtTag: %w", ErrFormat)
		}
		if _, ok := notNeedType[atype]; ok {
			continue
		}
		tags = append(tags, tag)
		anys = append(anys, v)
	}
	return tags, anys, nil
}
