package utils

import (
	"io/fs"
	"net/http"
	"regexp"
	"strings"

	"github.com/Masterminds/semver/v3"
	"github.com/xmdhs/clash2singbox/model"
)

var versionReg = regexp.MustCompile(`sing-box (\d+\.\d+\.\d+(-[0-9A-Za-z-.]+)?(\+[0-9A-Za-z-.]+)?)`)

func GetSingBoxVersion(r *http.Request) model.SingBoxVer {
	match := versionReg.FindStringSubmatch(r.UserAgent())
	if len(match) < 2 {
		return model.SINGLATEST
	}

	v, err := semver.NewVersion(match[1])
	if err != nil {
		return model.SINGLATEST
	}
	minor := v.Minor()

	switch {
	case minor <= 10:
		return model.SING110
	case minor == 11:
		return model.SING111
	case minor == 12:
		return model.SING112
	default:
		return model.SINGLATEST
	}
}

func GetConfig(v model.SingBoxVer, configFs fs.FS) []byte {
	switch {
	case v >= model.SING112:
		return FsReadAll(configFs, "config.json-1.12.0+.template")
	case v >= model.SING111:
		return FsReadAll(configFs, "config.json-1.11.0+.template")
	case v >= model.SING110:
		return FsReadAll(configFs, "config.json.template")
	default:
		return FsReadAll(configFs, "config.json-1.12.0+.template")
	}
}

// IsBrowser 检查 User-Agent 是否为浏览器
func IsBrowser(userAgent string) bool {
	ua := strings.ToLower(userAgent)
	return strings.Contains(ua, "mozilla")
}
