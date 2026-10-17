// Package session declares the module-nav session commands (Session: bookmarks, history, REPL);
// they register into nav/registry from init.
package session

import (
	"github.com/angeles725/niagara-go/internal/argparse"
	"github.com/angeles725/niagara-go/internal/nav/registry"
)

func init() {
	registry.Register("session", []registry.Command{
		{Name: "repl", Help: "Interactive REPL with all commands"},
		{Name: "bookmark", Help: "Save a class bookmark with optional note", Args: argsBookmark},
		{Name: "bookmarks", Help: "List all bookmarks", Args: argsBookmarks},
		{Name: "unbookmark", Help: "Remove a bookmark", Args: argsUnbookmark},
		{Name: "history", Help: "Show command history", Args: argsHistory},
		{Name: "session", Help: "Session save/load/export management", Args: argsSession},
	})
}

// argsBookmark declares `bookmark`.
func argsBookmark(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name to bookmark"}, "class_name")
	p.Add(argparse.Arg{Nargs: argparse.Optional, Help: "Optional note for the bookmark"}, "note")
}

// argsBookmarks declares `bookmarks`.
func argsBookmarks(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: "time", Choices: []any{"time", "name", "access"}, Help: "Sort order (default: time)", Dest: "bkmk_sort"}, "--sort")
	p.Add(argparse.Arg{Type: argparse.Int, Help: "Limit results"}, "-n")
}

// argsUnbookmark declares `unbookmark`.
func argsUnbookmark(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Class name to remove"}, "class_name")
}

// argsHistory declares `history`.
func argsHistory(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results (default 50)"}, "-n")
	p.Add(argparse.Arg{Action: argparse.StoreTrue, Help: "Show all history (up to 500)", Dest: "hist_all"}, "--all")
	p.Add(argparse.Arg{Help: "Filter by pattern", Dest: "hist_grep"}, "--grep")
}

// argsSession declares `session`.
func argsSession(p *argparse.Parser) {
	s := p.AddSubparsers(argparse.SubparsersConfig{Dest: "session_cmd", Help: "Session sub-commands"})
	argsSessionSave(s.AddParser("save", argparse.SubparserConfig{Help: "Save current bookmarks + history + notes as a session"}))
	argsSessionLoad(s.AddParser("load", argparse.SubparserConfig{Help: "Restore a saved session as active"}))
	argsSessionList(s.AddParser("list", argparse.SubparserConfig{Help: "List all saved sessions"}))
	argsSessionNote(s.AddParser("note", argparse.SubparserConfig{Help: "Add a note to the active session"}))
	argsSessionExport(s.AddParser("export", argparse.SubparserConfig{Help: "Export session to file or stdout"}))
	argsSessionDelete(s.AddParser("delete", argparse.SubparserConfig{Help: "Delete a saved session"}))
}

// argsSessionSave declares `session save`.
func argsSessionSave(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Session name"}, "name")
}

// argsSessionLoad declares `session load`.
func argsSessionLoad(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Session name"}, "name")
}

// argsSessionList declares `session list`.
func argsSessionList(p *argparse.Parser) {
	p.Add(argparse.Arg{Default: 50, Type: argparse.Int, Help: "Limit results"}, "-n")
}

// argsSessionNote declares `session note`.
func argsSessionNote(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Note text"}, "text")
}

// argsSessionExport declares `session export`.
func argsSessionExport(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Session name"}, "name")
	p.Add(argparse.Arg{Default: "md", Choices: []any{"md", "json"}, Help: "Export format (default: md)", Dest: "export_fmt"}, "--as")
	p.Add(argparse.Arg{Help: "Output file path", Dest: "export_out"}, "--out")
}

// argsSessionDelete declares `session delete`.
func argsSessionDelete(p *argparse.Parser) {
	p.Add(argparse.Arg{Help: "Session name"}, "name")
}
