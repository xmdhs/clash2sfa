package model

type ConvertArg struct {
	Sub       string
	Include   string
	Exclude   string
	Config    string
	ConfigUrl string
	AddTag    bool
	UrlTest   []UrlTestArg
}

type UrlTestArg struct {
	Tag       string `json:"tag"`
	Tolerance string `json:"tolerance"`
	Include   string `json:"include"`
	Exclude   string `json:"exclude"`
	Type      string `json:"type"`
}

type SingUrltest struct {
	Outbounds []string `json:"outbounds"`
	Tag       string   `json:"tag"`
	Tolerance int      `json:"tolerance,omitempty"`
	Type      string   `json:"type"`
}
