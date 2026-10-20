package course

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/angeles725/niagara-go/internal/cli"
)

// The quiz history is user state, not repo data (D4): one JSON record per graded answer, appended
// to $XDG_STATE_HOME/niagara-course/history.jsonl (~/.local/state/... when XDG_STATE_HOME is unset
// or relative), or to --history.
const (
	stateEnv    = "XDG_STATE_HOME"
	historyDir  = "niagara-course"
	historyName = "history.jsonl"
)

// The results a record carries: a skip is not graded, so it is not recorded.
const (
	resultRight = "right"
	resultWrong = "wrong"
)

// record is one graded answer of the history.
type record struct {
	Time   string `json:"time"` // RFC 3339, UTC
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Group  string `json:"group"`
	Result string `json:"result"` // resultRight or resultWrong
}

// at is the record's time; the zero time when it does not parse.
func (r record) at() time.Time {
	t, _ := time.Parse(time.RFC3339, r.Time)
	return t
}

// historyPath is --history when given, else the XDG state path read through s; "" and the message
// when neither the flag nor a home directory names one.
func historyPath(s cli.Stdio, flag string, hasFlag bool) (string, string) {
	if hasFlag {
		if flag == "" {
			return "", "--history: an empty path"
		}
		return flag, ""
	}
	if dir := s.Env(stateEnv); filepath.IsAbs(dir) {
		return filepath.Join(dir, historyDir, historyName), ""
	}
	home := s.Env("HOME")
	if home == "" && s.Getenv == nil {
		home, _ = os.UserHomeDir()
	}
	if home == "" {
		return "", "no history path: set --history, " + stateEnv + " or HOME"
	}
	return filepath.Join(home, ".local", "state", historyDir, historyName), ""
}

// appendRecord appends r to the history at path as one line, in one write to a file opened for
// appending (so concurrent quizzes never interleave inside a line), creating its directory.
func appendRecord(path string, r record) error {
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// loadHistory reads the records of the history at path, in file order; a missing file is an empty
// history. A line that is not a record (a write cut short) is skipped with a warning on stderr,
// so one bad line never locks the history; an unreadable file is an error.
func loadHistory(s cli.Stdio, path string) ([]record, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []record
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
	for n := 1; sc.Scan(); n++ {
		var r record
		if len(sc.Bytes()) == 0 {
			continue
		}
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil || r.ID == "" || r.at().IsZero() ||
			(r.Result != resultRight && r.Result != resultWrong) {
			fmt.Fprintf(s.Err, "course: history %s:%d: not a record, skipped\n", path, n)
			continue
		}
		out = append(out, r)
	}
	return out, sc.Err()
}
