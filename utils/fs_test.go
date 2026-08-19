package utils

import (
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFsReadAll(t *testing.T) {
	fsys := fstest.MapFS{
		"a.txt": &fstest.MapFile{Data: []byte("hello")},
	}
	assert.Equal(t, []byte("hello"), FsReadAll(fsys, "a.txt"))
}

func TestFsReadAllPanicsOnMissing(t *testing.T) {
	fsys := fstest.MapFS{}
	require.Panics(t, func() {
		FsReadAll(fsys, "missing.txt")
	})
}
