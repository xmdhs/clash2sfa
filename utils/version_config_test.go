package utils

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/xmdhs/clash2singbox/model"
)

func TestGetConfigByVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"config.json.template":         &fstest.MapFile{Data: []byte("v1.10")},
		"config.json-1.11.0+.template": &fstest.MapFile{Data: []byte("v1.11")},
		"config.json-1.12.0+.template": &fstest.MapFile{Data: []byte("v1.12")},
		"config.json-1.14.0+.template": &fstest.MapFile{Data: []byte("v1.14")},
	}

	cases := []struct {
		ver  model.SingBoxVer
		want string
	}{
		{model.SING110, "v1.10"},
		{model.SING111, "v1.11"},
		{model.SING112, "v1.12"},
		{model.SINGLATEST, "v1.14"},
		{model.SingBoxVer(9), "v1.14"},
	}
	for _, c := range cases {
		assert.Equal(t, []byte(c.want), GetConfig(c.ver, fsys), "ver=%v", c.ver)
	}
}
