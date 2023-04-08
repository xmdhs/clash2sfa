package handle

import (
	"net/http"
	"strconv"

	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/service"
)

func PutArg(db db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cxt := r.Context()
		c := r.FormValue("config")
		sub := r.FormValue("sub")
		if sub == "" {
			http.Error(w, "订阅链接不得为空", 400)
			return
		}
		include := r.FormValue("include")
		exclude := r.FormValue("exclude")

		arg := model.ConvertArg{
			Sub:     sub,
			Include: include,
			Exclude: exclude,
			Config:  c,
		}

		s, err := service.PutArg(cxt, arg, db)
		if err != nil {
			http.Error(w, err.Error(), 500)
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

func Sub(c *http.Client, db db.DB, frontendByte []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.FormValue("id")
		if id == "" {
			http.Error(w, "id 不得为空", 400)
			return
		}
		b, err := service.GetSub(r.Context(), c, db, id, frontendByte)
		if err != nil {
			http.Error(w, err.Error(), 500)
		}
		w.Write(b)
	}
}
