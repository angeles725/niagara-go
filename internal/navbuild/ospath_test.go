package navbuild

import "github.com/angeles725/niagara-go/internal/pycompat"

// isFile and isDir keep parity_test.go's spelling of os.path.isfile/isdir; that file
// belongs to the parity harness and is edited on its own branch.
var (
	isFile = pycompat.IsFile
	isDir  = pycompat.IsDir
)
