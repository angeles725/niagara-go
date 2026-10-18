package nav

import "github.com/angeles725/niagara-go/internal/nav/core"

// DetectBaseDir is detect_base_dir(): the checkout holding indexes/, or ""
// (core.LocateNavDir), with the error of a MODULE_NAVIGATOR_DIR or NAV_DIR
// that names no checkout.
func DetectBaseDir() (string, error) { return core.LocateNavDir() }
