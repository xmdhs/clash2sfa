package utils

import (
	"io"
	"io/fs"

	"github.com/samber/lo"
)

func FsReadAll(f fs.FS, fileName string) []byte {
	fh := lo.Must(f.Open(fileName))
	defer func() {
		_ = fh.Close()
	}()
	return lo.Must(io.ReadAll(fh))
}
