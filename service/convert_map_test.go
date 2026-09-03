package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xmdhs/clash2singbox/model/singbox"
)

func TestGetExtTagFromMap(t *testing.T) {
	config, err := decodeConfig([]byte(`{"outbounds":[{"type":"vmess","tag":"n1"},{"type":"direct","tag":"direct"}]}`))
	require.NoError(t, err)

	nodes, err := getExtTagFromMap(config)
	require.NoError(t, err)
	require.Len(t, nodes, 1)
	assert.Equal(t, "n1", nodes[0].tag)
	assert.Equal(t, "vmess", nodes[0].nodeType)
}

func TestUrlTestDetourSetFromMap(t *testing.T) {
	config, err := decodeConfig([]byte(`{"outbounds":[{"type":"selector","tag":"proxy","outbounds":["B"],"detour":"wrapper"}]}`))
	require.NoError(t, err)

	s := []singbox.SingBoxOut{{Type: "vmess", Tag: "B"}}
	outs := []map[string]any{{"type": "http", "tag": "wrapper", "server": "example.com"}}
	_, newOuts, extTags := urlTestDetourSetFromMap(s, nil, config, outs, nil)

	require.Len(t, newOuts, 2)
	assert.Equal(t, "B - wrapper [proxy]", newOuts[1]["tag"])
	assert.Equal(t, "B", newOuts[1]["detour"])
	require.Len(t, extTags, 1)
	assert.Equal(t, "B - wrapper [proxy]", extTags[0].Tag)
}

func TestDecodeConfigRejectsInvalidJSON(t *testing.T) {
	_, err := decodeConfig([]byte(`{`))
	assert.ErrorIs(t, err, ErrFormat)
}

func TestDecodeConfigKeepsJSONNumbers(t *testing.T) {
	config, err := decodeConfig([]byte(`{"outbounds":[{"type":"vmess","tag":"n1","server_port":443}]}`))
	require.NoError(t, err)
	encoded, err := json.Marshal(config)
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"server_port":443`)
}
