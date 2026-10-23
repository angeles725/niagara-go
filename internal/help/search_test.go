package help

import (
	"path/filepath"
	"testing"
)

// TestSearchHeaderCountsThePrintedRows: a name that several packages hold
// prints one row per class, and the header counts those rows, never fewer.
func TestSearchHeaderCountsThePrintedRows(t *testing.T) {
	const classIndex = `{"BA": {"package": "p.one", "type": "class", "others": [{"package": "p.two", "type": "interface"}]},` +
		`"BB": {"package": "p.one", "type": "class"}, "BC": {"package": "p.one", "type": "enum"}}`
	tests := []struct{ pattern, want string }{
		{"BA", "2 classes (1 name) matching 'BA':\n\n  BA (class) - p.one\n  BA (interface) - p.two\n"},
		{"B*", "4 classes (3 names) matching 'B*':\n\n  BA (class) - p.one\n  BA (interface) - p.two\n" +
			"  BB (class) - p.one\n  BC (enum) - p.one\n"},
		{"B[BC]", "2 classes matching 'B[BC]':\n\n  BB (class) - p.one\n  BC (enum) - p.one\n"},
	}
	for _, tt := range tests {
		t.Run(tt.pattern, func(t *testing.T) {
			dir := repo(t, twoVersions)
			write(t, filepath.Join(dir, "indexes", "class-index.json"), classIndex)
			out, errs, code := run(t, dir, "--niagara-version", "1.0", "search", tt.pattern)
			if code != 0 || errs != "" || out != tt.want {
				t.Errorf("code %d stderr %q stdout:\n%s\nwant:\n%s", code, errs, out, tt.want)
			}
		})
	}
}

// TestSearchQualifiedPatternListsOnlyMatchingPackages: a package-qualified glob
// prints only the classes whose package.name matches, never the other packages'
// classes that share a matching simple name.
func TestSearchQualifiedPatternListsOnlyMatchingPackages(t *testing.T) {
	const classIndex = `{"BRecipient": {"package": "javax.baja.alarm", "type": "class", "others": [{"package": "com.tridium.event", "type": "class"}]},` +
		`"BOther": {"package": "com.tridium.event", "type": "class"}}`
	dir := repo(t, twoVersions)
	write(t, filepath.Join(dir, "indexes", "class-index.json"), classIndex)
	out, errs, code := run(t, dir, "--niagara-version", "1.0", "search", "javax.baja.alarm*.B*")
	want := "1 classes matching 'javax.baja.alarm*.B*':\n\n  BRecipient (class) - javax.baja.alarm\n"
	if code != 0 || errs != "" || out != want {
		t.Errorf("code %d stderr %q stdout:\n%s\nwant:\n%s", code, errs, out, want)
	}
}
