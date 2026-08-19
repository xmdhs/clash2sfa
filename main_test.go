package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePortDefault(t *testing.T) {
	t.Setenv("PORT", "")
	t.Setenv("port", "")
	assert.Equal(t, ":8080", parsePort())
}

func TestParsePortUpper(t *testing.T) {
	t.Setenv("PORT", "5000")
	t.Setenv("port", "")
	assert.Equal(t, ":5000", parsePort())
}

func TestParsePortLower(t *testing.T) {
	t.Setenv("port", ":7000")
	t.Setenv("PORT", "")
	assert.Equal(t, ":7000", parsePort())
}

func TestParseLevelDefault(t *testing.T) {
	t.Setenv("level", "")
	assert.Equal(t, -4, parseLevel())
}

func TestParseLevelInvalid(t *testing.T) {
	t.Setenv("level", "not-a-number")
	assert.Equal(t, -4, parseLevel())
}

func TestParseLevelValid(t *testing.T) {
	t.Setenv("level", "0")
	assert.Equal(t, 0, parseLevel())
}

func TestNewServer(t *testing.T) {
	s := newServer(":9090", -4)
	require.NotNil(t, s)
	assert.Equal(t, ":9090", s.Addr)
	require.NotNil(t, s.Handler)
}
