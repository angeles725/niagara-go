package nav

import "github.com/angeles725/niagara-go/internal/nav/core"

// DetectBaseDir is detect_base_dir(): the checkout holding indexes/, or ""
// (core.FindNavDir).
func DetectBaseDir() string { return core.FindNavDir() }
