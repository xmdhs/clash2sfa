package handle

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/xmdhs/clash2sfa/db"
	"github.com/xmdhs/clash2sfa/model"
	"github.com/xmdhs/clash2sfa/service"
)

func PutArg(db db.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cxt := r.Context()

		arg := model.ConvertArg{}
		err := json.NewDecoder(r.Body).Decode(&arg)
		if err != nil {
			http.Error(w, err.Error(), 400)
		}
		if arg.Sub == "" {
			http.Error(w, "订阅链接不得为空", 400)
			return
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
