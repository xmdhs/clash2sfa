package utils

import (
	"io"
	"io/fs"

	"github.com/samber/lo"
)

func FsReadAll(f fs.FS, fileName string) []byte {
	return lo.Must(io.ReadAll(lo.Must(f.Open(fileName))))
}
