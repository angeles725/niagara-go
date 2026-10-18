package modulefind

import (
	"bytes"
	"testing"

	"github.com/angeles725/niagara-go/internal/cli"
)

// TestExitContract pins the D-I2-6 fixes the goldens do not reach: a compare srcB that holds
// no .java file is refused like <root>, an extends --of miss in JSON keeps the chain on stdout
// and reports the miss on stderr, and writers --csv with no static writer lists the dynamic ones.
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
