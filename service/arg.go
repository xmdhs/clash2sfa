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

	"github.com/tidwall/jsonc"
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

func (c *Convert) MakeConfig(cxt context.Context, arg model.ConvertArg, configByte []byte, userAgent string) ([]byte, error) {
	if arg.Config == nil && arg.ConfigUrl == "" {
		arg.Config = configByte
	}
	if arg.ConfigUrl != "" {
		b, err := httputils.HttpGet(cxt, c.c, arg.ConfigUrl, 1000*1000*10)
		if err != nil {
			return nil, fmt.Errorf("MakeConfig: %w", err)
		}
		arg.Config = b
	}
	// 支持 jsonc
	m, nodeTag, err := convert2sing(cxt, c.c, jsonc.ToJSON(arg.Config), arg.Sub, arg.Include, arg.Exclude, arg.AddTag, c.l, !arg.DisableUrlTest, arg.OutFields, arg.Ver)
	if err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	// convert2sing 产出的 outbounds 恒为 []any，此处不会失败；configUrlTestParser 的错误仅在直接调用时出现
	m, _ = configUrlTestParser(m, nodeTag)

	// 根据 User-Agent 决定是否格式化 JSON
	var result []byte
	if utils.IsBrowser(userAgent) {
		// 浏览器请求，返回格式化的 JSON；m 由 JSON 解析而来，序列化不会失败
		bw := &bytes.Buffer{}
		jw := json.NewEncoder(bw)
		jw.SetIndent("", "    ")
		_ = jw.Encode(m)
		result = bw.Bytes()
	} else {
		// 非浏览器请求，返回压缩的 JSON
		result, _ = json.Marshal(m)
	}

	return result, nil
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
	// 纯字面模式（无正则元字符）走 strings.Contains 快路径，
	// 省掉一次 Compile + RE2 执行。无任何跨请求状态。
	if isLiteralPattern(reg) {
		out := make([]string, 0, len(tags))
		for _, item := range tags {
			if strings.Contains(item, reg) == need {
				out = append(out, item)
			}
		}
		return out, nil
	}
	r, err := regexp.Compile(reg)
	if err != nil {
		return nil, fmt.Errorf("filter: %w", err)
	}
	out := make([]string, 0, len(tags))
	for _, item := range tags {
		if r.MatchString(item) == need {
			out = append(out, item)
		}
	}
	return out, nil
}

// isLiteralPattern 报告模式是否不含正则元字符。
func isLiteralPattern(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '.', '+', '*', '?', '(', ')', '|', '[', ']', '{', '}', '^', '$', '\\':
			return false
		}
	}
	return true
}

func configUrlTestParser(config map[string]any, tags []TagWithVisible) (map[string]any, error) {
	outL, ok := config["outbounds"].([]any)
	if !ok {
		return nil, fmt.Errorf("configUrlTestParser: outbounds is not []any or missing")
	}

	newOut := make([]any, 0, len(outL))

	for _, value := range outL {
		m, ok := value.(map[string]any)
		if !ok || m == nil {
			newOut = append(newOut, value)
			continue
		}
		outList, _ := m["outbounds"].([]any)
		if len(outList) == 0 {
			newOut = append(newOut, value)
			continue
		}

		tag, _ := m["tag"].(string)
		outListS := make([]string, 0, len(outList))
		for _, item := range outList {
			s, ok := item.(string)
			if ok {
				outListS = append(outListS, s)
			}
		}

		var tagStr []string
		if tag != "" {
			if detour, _ := m["detour"].(string); detour != "" {
				tagStr = make([]string, 0, len(tags))
				for _, item := range tags {
					if len(item.Visible) != 0 && slices.Contains(item.Visible, tag) {
						tagStr = append(tagStr, item.Tag)
					}
				}
				delete(m, "detour")
			} else {
				tagStr = make([]string, 0, len(tags))
				for _, item := range tags {
					if len(item.Visible) == 0 {
						tagStr = append(tagStr, item.Tag)
					}
				}
			}
		} else {
			tagStr = make([]string, 0, len(tags))
			for _, item := range tags {
				if len(item.Visible) == 0 {
					tagStr = append(tagStr, item.Tag)
				}
			}
		}

		tl, err := urlTestParser(outListS, tagStr)
		if err != nil {
			return nil, fmt.Errorf("configUrlTestParser: %w", err)
		}
		if tl != nil {
			m["outbounds"] = tl
		}
		newOut = append(newOut, value)
	}
	config["outbounds"] = newOut
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
		return nil, nil
	}

	tags, err := filterTags(tags, include, exclude)
	if err != nil {
		return nil, fmt.Errorf("urlTestParser: %w", err)
	}

	// 去重并保序：extTag 在前，tags 在后。避免 append(extTag, tags...) 别名底数组。
	seen := make(map[string]struct{}, len(extTag)+len(tags))
	out := make([]string, 0, len(extTag)+len(tags))
	for _, t := range extTag {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	for _, t := range tags {
		if _, ok := seen[t]; !ok {
			seen[t] = struct{}{}
			out = append(out, t)
		}
	}
	return out, nil
}
