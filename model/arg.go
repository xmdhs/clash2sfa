package model

import (
	"github.com/xmdhs/clash2singbox/model"
)

type ConvertArg struct {
	Sub            string
	Include        string
	Exclude        string
	Config         []byte
	ConfigUrl      string
	AddTag         bool
	DisableUrlTest bool
	OutFields      bool
	Ver            model.SingBoxVer
}
