package utils

import (
	"io/fs"

	"github.com/samber/lo"
)

func FsReadAll(f fs.FS, fileName string) []byte {
	return lo.Must(fs.ReadFile(f, fileName))
}
