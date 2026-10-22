package core

import (
	"archive/zip"
	"strings"

	"github.com/angeles725/niagara-go/internal/pycompat"
)

// ZipName is the name zipfile.ZipFile.namelist() gives an entry: UTF-8 when
// the entry's flag says so, else decoded as cp437, cut at the first NUL.
func ZipName(f *zip.File) string {
	n := f.Name
	if f.Flags&0x800 == 0 {
		n = pycompat.DecodeCP437(n)
	}
	if k := strings.IndexByte(n, 0); k >= 0 {
		n = n[:k]
	}
	return n
}
