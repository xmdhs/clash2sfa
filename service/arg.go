package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"log/slog"

	"github.com/samber/lo"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/utils"
	"github.com/xmdhs/clash2singbox/httputils"
)

type Convert struct {
	c *http.Client
	l *slog.Logger
}

func NewConvert(c *http.Client, l *slog.Logger) *Convert {
	return &Convert{
		c: c,
		l: l,
	}
}

func (c *Convert) MakeConfig(cxt context.Context, arg model.ConvertArg, configByte []byte) ([]byte, error) {
	if arg.Config == "" && arg.ConfigUrl == "" {
		arg.Config = string(configByte)
	}
	if arg.ConfigUrl != "" {
		b, err := httputils.HttpGet(cxt, c.c, arg.ConfigUrl, 1000*1000*10)
		if err != nil {
			return nil, fmt.Errorf("MakeConfig: %w", err)
		}
		arg.Config = string(b)
	}
	m, nodeTag, err := convert2sing(cxt, c.c, arg.Config, arg.Sub, arg.Include, arg.Exclude, arg.AddTag, c.l, !arg.DisableUrlTest, arg.OutFields, arg.Ver)
	if err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	m, err = configUrlTestParser(m, nodeTag)
	if err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	bw := &bytes.Buffer{}
	jw := json.NewEncoder(bw)
	jw.SetIndent("", "    ")
	err = jw.Encode(m)
	if err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	return bw.Bytes(), nil
}

var (
	ErrJson = errors.New("错误的 json")
)

func filterTags(tags []string, include, exclude string) ([]string, error) {
	nt, err := filter(include, tags, true)
	if err != nil {
		return nil, fmt.Errorf("filterTags: %w", err)
	}
	nt, err = filter(exclude, nt, false)
	if err != nil {
		return nil, fmt.Errorf("filterTags: %w", err)
	}
	return nt, nil
}

func filter(reg string, tags []string, need bool) ([]string, error) {
	if reg == "" {
		return tags, nil
	}
	r, err := regexp.Compile(reg)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	tag := lo.Filter[string](tags, func(item string, index int) bool {
		has := r.MatchString(item)
		return has == need
	})
	return tag, nil
}

func configUrlTestParser(config map[string]any, tags []string) (map[string]any, error) {
	outL := config["outbounds"].([]any)

	newOut := make([]any, 0, len(outL))

	for _, value := range outL {
		outList := utils.AnyGet[[]any](value, "outbounds")

		if len(outList) == 0 {
			newOut = append(newOut, value)
			continue
		}
		outListS := lo.FilterMap(outList, func(item any, index int) (string, bool) {
			s, ok := item.(string)
			return s, ok
		})
		tl, err := urlTestParser(outListS, tags)
		if err != nil {
			return nil, fmt.Errorf("customUrlTest: %w", err)
		}
		if tl == nil {
			newOut = append(newOut, value)
			continue
		}
		utils.AnySet(&value, tl, "outbounds")
		newOut = append(newOut, value)
	}
	utils.AnySet(&config, filterNilOutBonds(newOut, tags), "outbounds")
	return config, nil
}

func urlTestParser(outbounds, tags []string) ([]string, error) {
	var include, exclude string
	extTag := []string{}

	for _, s := range outbounds {
		if after, ok := strings.CutPrefix(s, "include: "); ok {
			include = after
		} else if after, ok := strings.CutPrefix(s, "exclude: "); ok {
			exclude = after
		} else {
			extTag = append(extTag, s)
		}
	}

	if include == "" && exclude == "" {
		if len(extTag) != 0 {
			return extTag, nil
		}
		return nil, nil
	}

	tags, err := filterTags(tags, include, exclude)
	if err != nil {
		return nil, fmt.Errorf("urlTestParser: %w", err)
	}

	return lo.Union(append(extTag, tags...)), nil
}

func filterNilOutBonds(outL []any, tags []string) []any {
	newList := make([]any, 0, len(outL))
	tagM := make(map[string]struct{})
	for _, v := range tags {
		tagM[v] = struct{}{}
	}
	for _, v := range outL {
		if tag := utils.AnyGet[string](v, "tag"); tag != "" {
			tagM[tag] = struct{}{}
		}
	}

	for _, value := range outL {
		atype := utils.AnyGet[string](value, "type")
		if atype == "urltest" || atype == "selector" {
			outList := utils.AnyGet[[]string](value, "outbounds")
			if len(outList) == 0 {
				delete(tagM, utils.AnyGet[string](value, "tag"))
				continue
			}
			filteredList := lo.Filter(outList, func(item string, i int) bool {
				_, ok := tagM[item]
				return ok
			})
			if len(filteredList) == 0 {
				delete(tagM, utils.AnyGet[string](value, "tag"))
				continue
			}
			if len(filteredList) != len(outList) {
				utils.AnySet(&value, filteredList, "outbounds")
			}
			newList = append(newList, value)
		} else {
			newList = append(newList, value)
		}
	}
	return newList
}
