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
	"strconv"

	"log/slog"

	"github.com/samber/lo"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2singbox/httputils"
	"lukechampine.com/blake3"
)

func PutArg(cxt context.Context, arg model.ConvertArg, db db.DB) (string, error) {
	b, err := json.Marshal(arg)
	if err != nil {
		return "", fmt.Errorf("PutArg: %w", err)
	}
	hash := blake3.Sum256(b)
	h := hex.EncodeToString(hash[:])
	err = db.PutArg(cxt, h, arg)
	if err != nil {
		return "", fmt.Errorf("PutArg: %w", err)
	}
	return h, nil
}

func GetSub(cxt context.Context, c *http.Client, db db.DB, id string, frontendByte []byte, l *slog.Logger) ([]byte, error) {
	arg, err := db.GetArg(cxt, id)
	if err != nil {
		return nil, fmt.Errorf("GetSub: %w", err)
	}
	b, err := MakeConfig(cxt, c, frontendByte, l, arg)
	if err != nil {
		return nil, fmt.Errorf("GetSub: %w", err)
	}
	return b, nil
}

func MakeConfig(cxt context.Context, c *http.Client, frontendByte []byte, l *slog.Logger, arg model.ConvertArg) ([]byte, error) {
	if arg.Config == "" && arg.ConfigUrl == "" {
		arg.Config = string(frontendByte)
	}
	if arg.ConfigUrl != "" {
		b, err := httputils.HttpGet(cxt, c, arg.ConfigUrl, 1000*1000*10)
		if err != nil {
			return nil, fmt.Errorf("MakeConfig: %w", err)
		}
		arg.Config = string(b)
	}
	b, err := convert2sing(cxt, c, arg.Config, arg.Sub, arg.Include, arg.Exclude, l)
	if err != nil {
		return nil, fmt.Errorf("MakeConfig: %w", err)
	}
	if len(arg.UrlTest) != 0 {
		nb, err := customUrlTest(b, arg.UrlTest)
		if err != nil {
			return nil, fmt.Errorf("MakeConfig: %w", err)
		}
		b = nb
	}
	return b, nil
}

var (
	ErrJson = errors.New("错误的 json")
)

func customUrlTest(config []byte, u []model.UrlTestArg) ([]byte, error) {
	r := gjson.GetBytes(config, `outbounds.#(tag=="urltest").outbounds`)
	if !r.Exists() {
		return nil, fmt.Errorf("customUrlTest: %w", ErrJson)
	}
	sl := []model.SingUrltest{}

	tags := []string{}
	r.ForEach(func(key, value gjson.Result) bool {
		tags = append(tags, value.String())
		return true
	})

	for _, v := range u {
		nt, err := filter(v.Include, tags, true)
		if err != nil {
			return nil, fmt.Errorf("customUrlTest: %w", err)
		}
		nt, err = filter(v.Exclude, nt, false)
		if err != nil {
			return nil, fmt.Errorf("customUrlTest: %w", err)
		}
		t, _ := lo.TryOr[int](func() (int, error) { return strconv.Atoi(v.Tolerance) }, 0)

		sl = append(sl, model.SingUrltest{
			Outbounds: nt,
			Tag:       v.Tag,
			Tolerance: t,
			Type:      "urltest",
		})
	}

	var b []byte
	for _, v := range sl {
		var err error
		b, err = sjson.SetBytes(config, "outbounds.-1", v)
		if err != nil {
			return nil, fmt.Errorf("customUrlTest: %w", err)
		}
	}
	var a any
	lo.Must0(json.Unmarshal(b, &a))
	bw := bytes.NewBuffer(nil)
	jw := json.NewEncoder(bw)
	jw.SetEscapeHTML(false)
	jw.SetIndent("", "    ")
	lo.Must0(jw.Encode(a))
	return bw.Bytes(), nil
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
