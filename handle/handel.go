package handle

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"log/slog"

	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/service"
)

type Handle struct {
	convert *service.Convert
	l       *slog.Logger
}

func NewHandle(convert *service.Convert, l *slog.Logger) *Handle {
	return &Handle{
		convert: convert,
		l:       l,
	}
}

func (h *Handle) PutArg(w http.ResponseWriter, r *http.Request) {
	cxt := r.Context()
	arg := model.ConvertArg{}
	err := json.NewDecoder(r.Body).Decode(&arg)
	if err != nil {
		h.l.DebugContext(cxt, err.Error())
		http.Error(w, err.Error(), 400)
		return
	}
	if arg.Sub == "" {
		h.l.DebugContext(cxt, "订阅链接不得为空")
		http.Error(w, "订阅链接不得为空", 400)
		return
	}
	s, err := h.convert.PutArg(cxt, arg)
	if err != nil {
		h.l.WarnContext(cxt, err.Error())
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write([]byte(s))
}

func Frontend(frontendByte []byte, age int) http.HandlerFunc {
	sage := strconv.Itoa(age)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age="+sage)
		w.Write(frontendByte)
	}
}

func (h *Handle) Sub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.FormValue("id")
	config := r.FormValue("config")
	curl := r.FormValue("configurl")
	sub := r.FormValue("sub")
	include := r.FormValue("include")
	exclude := r.FormValue("exclude")
	addTag := r.FormValue("addTag")
	disableUrlTest := r.FormValue("disableUrlTest")
	disableUrlTestb := false
	addTagb := false

	if id == "" && sub == "" {
		h.l.DebugContext(ctx, "id 不得为空")
		http.Error(w, "id 不得为空", 400)
		return
	}
	if addTag == "true" {
		addTagb = true
	}
	if disableUrlTest == "true" {
		disableUrlTestb = true
	}

	rc := http.NewResponseController(w)
	rc.SetWriteDeadline(time.Now().Add(2 * time.Minute))

	b, err := func() ([]byte, error) {
		if sub != "" {
			if config != "" {
				b, err := zlibDecode(config)
				if err != nil {
					return nil, err
				}
				config = string(b)
			}
			a := model.ConvertArg{
				Sub:            sub,
				Include:        include,
				Exclude:        exclude,
				Config:         config,
				ConfigUrl:      curl,
				AddTag:         addTagb,
				DisableUrlTest: disableUrlTestb,
			}
			return h.convert.MakeConfig(ctx, a)
		}
		return h.convert.GetSub(ctx, id)
	}()
	if err != nil {
		h.l.WarnContext(ctx, err.Error())
		http.Error(w, err.Error(), 500)
		return
	}
	w.Write(b)

}

func zlibDecode(s string) ([]byte, error) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	r, err := zlib.NewReader(bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	b, err = io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return b, nil
}
