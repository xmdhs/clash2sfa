package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"strings"

	"log/slog"

	"github.com/samber/lo"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/utils"
	"github.com/xmdhs/clash2singbox/httputils"
)

type Convert struct {
	c          *http.Client
	configByte []byte
	l          *slog.Logger
}

func NewConvert(c *http.Client, frontendByte []byte, l *slog.Logger) *Convert {
	return &Convert{
		c:          c,
		configByte: frontendByte,
		l:          l,
	}
}

func (c *Convert) MakeConfig(cxt context.Context, arg model.ConvertArg) ([]byte, error) {
	if arg.Config == "" && arg.ConfigUrl == "" {
		arg.Config = string(c.configByte)
	}
	if arg.ConfigUrl != "" {
		b, err := httputils.HttpGet(cxt, c.c, arg.ConfigUrl, 1000*1000*10)
		if err != nil {
			return nil, fmt.Errorf("MakeConfig: %w", err)
		}
		arg.Config = string(b)
	}
	m, nodeTag, err := convert2sing(cxt, c.c, arg.Config, arg.Sub, arg.Include, arg.Exclude, arg.AddTag, c.l, !arg.DisableUrlTest)
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

func configUrlTestParser(config map[string]any, tags []TagWithVisible) (map[string]any, error) {
	outL := config["outbounds"].([]any)

	newOut := make([]any, 0, len(outL))

	for _, value := range outL {
		value := value

		outList := utils.AnyGet[[]any](value, "outbounds")

		if len(outList) == 0 {
			newOut = append(newOut, value)
			continue
		}

		tag := utils.AnyGet[string](value, "tag")

		outListS := lo.FilterMap(outList, func(item any, index int) (string, bool) {
			s, ok := item.(string)
			return s, ok
		})

		var tagStr []string

		if utils.AnyGet[string](value, "detour") != "" {
			tagStr = lo.FilterMap(tags, func(item TagWithVisible, index int) (string, bool) {
				return item.Tag, len(item.Visible) != 0 && slices.Contains(item.Visible, tag)
			})
			m, ok := value.(map[string]any)
			if ok {
				delete(m, "detour")
			}
		} else {
			tagStr = lo.FilterMap(tags, func(item TagWithVisible, index int) (string, bool) {
				return item.Tag, len(item.Visible) == 0
			})
		}

		tl, err := urlTestParser(outListS, tagStr)
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
	utils.AnySet(&config, newOut, "outbounds")
	return config, nil
}

func urlTestParser(outbounds, tags []string) ([]string, error) {
	var include, exclude string
	extTag := []string{}

	for _, s := range outbounds {
		if strings.HasPrefix(s, "include: ") {
			include = strings.TrimPrefix(s, "include: ")
		} else if strings.HasPrefix(s, "exclude: ") {
			exclude = strings.TrimPrefix(s, "exclude: ")
		} else {
			extTag = append(extTag, s)
		}
	}

	if include == "" && exclude == "" {
		return nil, nil
	}

	tags, err := filterTags(tags, include, exclude)
	if err != nil {
		return nil, fmt.Errorf("urlTestParser: %w", err)
	}

	return lo.Union(append(extTag, tags...)), nil
}
