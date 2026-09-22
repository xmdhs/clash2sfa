package handle

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/service"
	"github.com/xmdhs/clash2sfa/utils"
	cmodel "github.com/xmdhs/clash2singbox/model"
)

type Handle struct {
	convert  *service.Convert
	l        *slog.Logger
	configFs fs.FS
}

func NewHandle(convert *service.Convert, l *slog.Logger, configFs fs.FS) *Handle {
	return &Handle{convert: convert, l: l, configFs: configFs}
}

// Frontend 返回启动时渲染好的首页。
func Frontend(frontendByte []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(frontendByte)
	}
}

// Sub 处理 /sub：按查询参数抓取订阅并返回转换后的 sing-box 配置。
//
// 参数：sub 订阅地址（必填）；include / exclude 是默认 urltest 的节点过滤正则；
// config 是经 zlib 压缩再 base64url 编码的模板；configurl 是模板地址，不以 http 开头时视为内置模板文件名；
// ua 非空时抓取订阅与远程模板改用该 User-Agent；
// addTag / disableUrlTest 为 "true" 时生效；outFields 为 "1" / "0" 时强制开启 / 关闭 block 与 dns-out 的生成。
func (h *Handle) Sub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sub := r.FormValue("sub")
	if sub == "" {
		h.l.DebugContext(ctx, "sub 不得为空")
		http.Error(w, "sub 不得为空", http.StatusBadRequest)
		return
	}

	ver := utils.GetSingBoxVersion(r)
	arg := model.ConvertArg{
		Sub:            sub,
		Include:        r.FormValue("include"),
		Exclude:        r.FormValue("exclude"),
		ConfigUrl:      r.FormValue("configurl"),
		AddTag:         r.FormValue("addTag") == "true",
		DisableUrlTest: r.FormValue("disableUrlTest") == "true",
		OutFields:      outFields(r.FormValue("outFields"), ver),
		Ver:            ver,
		UserAgent:      r.FormValue("ua"),
	}

	if arg.ConfigUrl != "" && !strings.HasPrefix(arg.ConfigUrl, "http") {
		b, err := fs.ReadFile(h.configFs, arg.ConfigUrl)
		if err != nil {
			h.l.WarnContext(ctx, err.Error())
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		arg.Config = b
		arg.ConfigUrl = ""
	}
	if config := r.FormValue("config"); config != "" {
		b, err := zlibDecode(config)
		if err != nil {
			h.l.WarnContext(ctx, err.Error())
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		arg.Config = b
	}

	// 拉取多个订阅可能较慢，放宽 server 级别的 WriteTimeout
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(2 * time.Minute))

	b, err := h.convert.MakeConfig(ctx, arg, utils.GetConfig(ver, h.configFs), r.UserAgent())
	if err != nil {
		h.l.WarnContext(ctx, err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(b)
}

// outFields 决定是否生成 block / dns-out 这类旧版 outbound：显式参数优先，否则只有 sing-box 1.10 及更早版本需要。
func outFields(param string, ver cmodel.SingBoxVer) bool {
	switch param {
	case "1":
		return true
	case "0":
		return false
	}
	return ver <= cmodel.SING110
}

// maxDecompressedConfig 是 config 参数解压后的上限，与订阅抓取的 10MB 对齐，防止 zip bomb。
const maxDecompressedConfig = 10 * 1000 * 1000

// zlibDecode 解码 config 参数：base64url(zlib(模板))。
func zlibDecode(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	b, err = io.ReadAll(io.LimitReader(r, maxDecompressedConfig+1))
	if err != nil {
		return nil, err
	}
	if len(b) > maxDecompressedConfig {
		return nil, errors.New("zlibDecode: 解压后配置过大")
	}
	return b, nil
}
