package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"

	"log/slog"

	"github.com/samber/lo"
	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/utils"
	"github.com/xmdhs/clash2singbox/httputils"
	"lukechampine.com/blake3"
)

type Convert struct {
	c          *http.Client
	db         db.DB
	configByte []byte
	l          *slog.Logger
}

func NewConvert(c *http.Client, db db.DB, frontendByte []byte, l *slog.Logger) *Convert {
	return &Convert{
		c:          c,
		db:         db,
		configByte: frontendByte,
		l:          l,
	}
}

func (c *Convert) PutArg(cxt context.Context, arg model.ConvertArg) (string, error) {
	b, err := json.Marshal(arg)
	if err != nil {
		return "", fmt.Errorf("PutArg: %w", err)
	}
	hash := blake3.Sum256(b)
	h := hex.EncodeToString(hash[:])
	err = c.db.PutArg(cxt, h, arg)
	if err != nil {
		return "", fmt.Errorf("PutArg: %w", err)
	}
	return h, nil
}

func (c *Convert) GetSub(cxt context.Context, id string) ([]byte, error) {
	arg, err := c.db.GetArg(cxt, id)
	if err != nil {
		return nil, fmt.Errorf("GetSub: %w", err)
	}
	b, err := c.MakeConfig(cxt, arg)
	if err != nil {
		return nil, fmt.Errorf("GetSub: %w", err)
	}
	return b, nil
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

func configUrlTestParser(config map[string]any, tags []string) (map[string]any, error) {
	outL := config["outbounds"].([]any)

	newOut := make([]any, 0, len(outL))

	for _, value := range outL {
		value := value

		outList := utils.AnyGet[[]any](value, "outbounds")

		if len(outList) == 0 {
			newOut = append(newOut, value)
			continue
		}

		outListS := lo.FilterMap[any, string](outList, func(item any, index int) (string, bool) {
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
