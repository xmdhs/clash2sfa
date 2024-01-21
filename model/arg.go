package model

type ConvertArg struct {
	Sub            string
	Include        string
	Exclude        string
	Config         string
	ConfigUrl      string
	AddTag         bool
	DisableUrlTest bool
}
