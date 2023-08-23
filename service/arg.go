package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"

	"log/slog"

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
	return b, nil
}
