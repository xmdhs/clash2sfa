package service

import (
	"encoding/json"
	"strings"
	"testing"
)

// FuzzGetExtTag 任意配置字节解析都不应 panic。
func FuzzGetExtTag(f *testing.F) {
	seeds := []string{
		`{"outbounds":[{"type":"vmess","tag":"a"},{"type":"direct","tag":"direct"}]}`,
		`{}`,
		`{`,
		``,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = getExtTag(data)
	})
}

// FuzzUrlTestParser 任意 outbounds 列表过滤都不应 panic（换行分隔表达列表）。
func FuzzUrlTestParser(f *testing.F) {
	f.Add("include: HK\ndirect")
	f.Add("exclude: 01\ndirect\nblock")
	f.Add("include: [\ndirect")
	f.Add("")
	f.Fuzz(func(t *testing.T, data string) {
		_, _ = urlTestParser(strings.Split(data, "\n"), []string{"HK-01", "JP-01", "SG-01"})
	})
}

// FuzzFilterTags 任意过滤表达式与标签都不应 panic（标签用换行分隔）。
func FuzzFilterTags(f *testing.F) {
	f.Add("HK", "01", "HK-01\nJP-01")
	f.Add("[", "", "a")
	f.Add("", "", "")
	f.Fuzz(func(t *testing.T, include, exclude, tagsData string) {
		_, _ = filterTags(strings.Split(tagsData, "\n"), include, exclude)
	})
}

// FuzzConfigUrlTestParser 任意配置 map 都不应 panic。
func FuzzConfigUrlTestParser(f *testing.F) {
	seeds := []string{
		`{"outbounds":[]}`,
		`{"outbounds":[{"type":"selector","tag":"p","outbounds":["include: HK"],"detour":"x"}]}`,
		`{"outbounds":[{"type":"selector","tag":"p","outbounds":["direct","direct"]}]}`,
		`{"outbounds":"not-slice"}`,
		`{`,
	}
	for _, s := range seeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var m map[string]any
		if json.Unmarshal(data, &m) != nil {
			return
		}
		_, _ = configUrlTestParser(m, nil)
	})
}
