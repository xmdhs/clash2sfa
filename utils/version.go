package utils

import (
	"net/http"
	"regexp"

	"github.com/Masterminds/semver/v3"
)

var versionReg = regexp.MustCompile(`sing-box (\d+\.\d+\.\d+(-[0-9A-Za-z-.]+)?(\+[0-9A-Za-z-.]+)?)`)

func GetSingBoxVersion(r *http.Request) *semver.Version {
	l := versionReg.FindStringSubmatch(r.UserAgent())
	if len(l) >= 2 {
		v, _ := semver.NewVersion(l[1])
		return v
	}
	return nil
}
