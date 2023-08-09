package handle

import (
	"encoding/json"
	"net/http"
	"strconv"

	"log/slog"

	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/service"
)

func PutArg(db db.DB, l *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cxt := r.Context()

		arg := model.ConvertArg{}
		err := json.NewDecoder(r.Body).Decode(&arg)
		if err != nil {
			l.DebugContext(cxt, err.Error())
			http.Error(w, err.Error(), 400)
			return
		}
		if arg.Sub == "" {
			l.DebugContext(cxt, "订阅链接不得为空")
			http.Error(w, "订阅链接不得为空", 400)
			return
		}
		s, err := service.PutArg(cxt, arg, db)
		if err != nil {
			l.WarnContext(cxt, err.Error())
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write([]byte(s))
	}
}

func Frontend(frontendByte []byte, age int) http.HandlerFunc {
	sage := strconv.Itoa(age)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age="+sage)
		w.Write(frontendByte)
	}
}

func Sub(c *http.Client, db db.DB, frontendByte []byte, l *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		id := r.FormValue("id")
		if id == "" {
			l.DebugContext(ctx, "id 不得为空")
			http.Error(w, "id 不得为空", 400)
			return
		}
		b, err := service.GetSub(r.Context(), c, db, id, frontendByte, l)
		if err != nil {
			l.WarnContext(ctx, err.Error())
			http.Error(w, err.Error(), 500)
			return
		}
		w.Write(b)
	}
}
