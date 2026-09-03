package service

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/xmdhs/clash2sfa/model"
	cmodel "github.com/xmdhs/clash2singbox/model"
)

func benchmarkSfaTemplateJSON(nodeCount int) []byte {
	var b bytes.Buffer
	b.WriteString(`{"outbounds":[`)
	for i := range nodeCount {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"type":"http","tag":"template-%d","server":"example.com","server_port":443}`, i)
	}
	b.WriteString(`]}`)
	return b.Bytes()
}

func BenchmarkGetExtTag(b *testing.B) {
	config := benchmarkSfaTemplateJSON(1000)
	b.ReportAllocs()
	b.SetBytes(int64(len(config)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := getExtTag(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkGetExtTagFromMap(b *testing.B) {
	config := benchmarkSfaTemplateJSON(1000)
	decoded, err := decodeConfig(config)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(config)))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := getExtTagFromMap(decoded)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMakeConfig(b *testing.B) {
	client := &http.Client{Transport: staticRT{body: []byte(subYAML)}}
	converter := NewConvert(client, newSilentLogger())
	arg := model.ConvertArg{
		Sub:    "https://example.com/sub",
		Config: benchmarkSfaTemplateJSON(100),
		Ver:    cmodel.SINGLATEST,
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := converter.MakeConfig(context.Background(), arg, nil, "sing-box/2.0"); err != nil {
			b.Fatal(err)
		}
	}
}
