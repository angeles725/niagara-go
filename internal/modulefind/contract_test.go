package modulefind

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// TestExitContract pins the D-I2-6 fixes the goldens do not reach: a compare srcB that holds
// no .java file is refused like <root>, an extends --of miss in JSON keeps the chain on stdout
// and reports the miss on stderr (extends --csv follows the JSON shape), and writers --csv with
// no static writer lists the dynamic ones.
func TestExitContract(t *testing.T) {
	t.Chdir("testdata")
	for _, tt := range []struct {
		name           string
		args           []string
		stdout, stderr string
		exit           int
	}{
		{name: "compare srcB without java", args: []string{"srcA", "compare", "nojava"},
			stderr: "module-find: no .java files under nojava (dot-dirs pruned)\n", exit: 3},
		{name: "compare srcB that is a file", args: []string{"srcA", "compare", "nojava/README.txt"},
			stderr: "module-find: not a directory: nojava/README.txt\n", exit: 3},
		{name: "extends --of miss as JSON", args: []string{"srcA", "extends", "--of", "Nope", "--json"},
			stdout: "{\n  \"class\": \"Nope\",\n  \"chain\": [\n    \"Nope\"\n  ]\n}\n",
			stderr: "(no extends declaration for Nope) -- searched srcA: 4 java files\n", exit: 1},
		{name: "extends --of hit", args: []string{"srcA", "extends", "--of", "BColdRoom"},
			stdout: "BColdRoom -> BComponent\n"},
		{name: "extends --csv", args: []string{"srcA", "extends", "--csv"},
			stdout: "class,superclass\r\nBColdRoom,BComponent\r\nBDashServlet,BWebServlet\r\nBRoomPanel,BColdRoom\r\n"},
		{name: "extends --of --csv", args: []string{"srcA", "extends", "--of", "BRoomPanel", "--csv"},
			stdout: "class,position,ancestor\r\nBRoomPanel,0,BRoomPanel\r\nBRoomPanel,1,BColdRoom\r\nBRoomPanel,2,BComponent\r\n"},
		{name: "extends --of miss as CSV", args: []string{"srcA", "extends", "--of", "Nope", "--csv"},
			stdout: "class,position,ancestor\r\nNope,0,Nope\r\n",
			stderr: "(no extends declaration for Nope) -- searched srcA: 4 java files\n", exit: 1},
		{name: "writers --csv with only dynamic writers", args: []string{"srcA", "writers", "nobody", "--csv"},
			stdout: "slot,kind,writer\r\nnobody,dynamic,\"BDashServlet: comp.set(prop, ...)  [slot resolved at runtime]\"\r\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := Main(tt.args, cli.Stdio{Out: &out, Err: &errb})
			if code != tt.exit || out.String() != tt.stdout || errb.String() != tt.stderr {
				t.Errorf("exit %d stdout %q stderr %q\nwant exit %d stdout %q stderr %q",
					code, out.String(), errb.String(), tt.exit, tt.stdout, tt.stderr)
			}
		})
	}
}

// plainTree is a module root holding one .java file that declares no slot, action, extends,
// ORD or writer: every command searches it and finds nothing.
func plainTree(t *testing.T) {
	t.Helper()
	t.Chdir(t.TempDir())
	if err := os.MkdirAll(filepath.Join("plain", "x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join("plain", "x", "Plain.java"), []byte("package x;\npublic class Plain {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestNonSearchCommandsExitZeroWhenEmpty pins the run convention: a command that lists or
// summarizes (it returns -1) exits 0 even when it has nothing to list; only a search that
// matched nothing (it returns 0) exits 1.
func TestNonSearchCommandsExitZeroWhenEmpty(t *testing.T) {
	plainTree(t)
	for _, args := range [][]string{
		{"plain", "slot-types"},
		{"plain", "slot-types", "--json"},
		{"plain", "ext-writable"},
		{"plain", "ext-writable", "--csv"},
		{"plain", "extends"},
		{"plain", "extends", "--json"},
		{"plain", "compare", "plain"},
		{"selftest"},
	} {
		var out, errb bytes.Buffer
		if code := Main(args, cli.Stdio{Out: &out, Err: &errb}); code != 0 {
			t.Errorf("%v: exit %d, want 0 (stdout %q stderr %q)", args, code, out.String(), errb.String())
		}
	}
}

// TestWritersMiss pins writers under the exit contract: a slot with no static and no dynamic
// writer is a searched miss (exit 1, the miss line names the scope); a dynamic writer alone is
// a hit, since it may write any slot.
func TestWritersMiss(t *testing.T) {
	plainTree(t)
	const scope = " -- searched plain: 1 java files\n"
	for _, tt := range []struct {
		name           string
		args           []string
		stdout, stderr string
	}{
		{name: "text", args: []string{"plain", "writers", "foo"},
			stdout: "foo: no STATIC writer (.set(\"foo\",...) or setFoo(...))" + scope},
		{name: "json", args: []string{"plain", "writers", "foo", "--json"},
			stdout: "{\n  \"slot\": \"foo\",\n  \"writers\": [],\n  \"dynamic\": []\n}\n",
			stderr: "(no writers of foo)" + scope},
		{name: "csv", args: []string{"plain", "writers", "foo", "--csv"},
			stderr: "(no writers of foo)" + scope},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var out, errb bytes.Buffer
			code := Main(tt.args, cli.Stdio{Out: &out, Err: &errb})
			if code != 1 || out.String() != tt.stdout || errb.String() != tt.stderr {
				t.Errorf("exit %d stdout %q stderr %q\nwant exit 1 stdout %q stderr %q",
					code, out.String(), errb.String(), tt.stdout, tt.stderr)
			}
		})
	}
}
