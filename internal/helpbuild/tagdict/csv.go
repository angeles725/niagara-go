package tagdict

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"slices"
	"strings"
)

// The CSV tables of haystack-rt name their columns on a header row whose
// first cell is "name" (h3Name for the migration table), under a row naming
// column groups ("impliedTagRules", "Validity Rules") that hold from where
// they appear to the right; rows above them (tags.csv's version row) are not
// read. A header cell left empty takes its group's name (4.3's units).

// topLevel are the columns that are not rule cells, whatever group they sit in.
var topLevel = []string{"name", "kind", "SmartType", "units", "Remove", "tags"}

// column is one column of a table: its header and the group it sits under.
type column struct{ name, group string }

// table is a CSV table: its columns and the rows below the header.
type table struct {
	cols []column
	rows [][]string
}

// readTable finds the header row (first cell "name" or "h3Name") of data.
func readTable(data []byte) (*table, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	recs, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	for i, rec := range recs {
		if rec[0] != "name" && rec[0] != "h3Name" {
			continue
		}
		t := &table{rows: recs[i+1:]}
		group := ""
		for c, h := range rec {
			if i > 0 && c < len(recs[i-1]) && recs[i-1][c] != "" {
				group = recs[i-1][c]
			}
			if h == "" {
				h = group
			}
			t.cols = append(t.cols, column{h, group})
		}
		return t, nil
	}
	return nil, errors.New("no header row (a first cell \"name\" or \"h3Name\")")
}

// has reports whether the table has a column named name.
func (t *table) has(name string) bool {
	return slices.ContainsFunc(t.cols, func(c column) bool { return c.name == name })
}

// cells are the named non-empty cells of row, top-level and rule ones apart;
// a rule cell is listed under the group it sits in.
func (t *table) cells(row []string) (top map[string]string, rules map[string][]Field) {
	top, rules = map[string]string{}, map[string][]Field{}
	for c, v := range row {
		if v = strings.TrimSpace(v); v == "" || c >= len(t.cols) {
			continue
		}
		col := t.cols[c]
		if slices.Contains(topLevel, col.name) || col.group == "" || col.name == col.group {
			top[col.name] = v
		} else {
			rules[col.group] = append(rules[col.group], Field{col.name, v})
		}
	}
	return top, rules
}

// readCSV reads tags.csv or smartRefsImport.csv (a kind column), equip.csv
// (a tags column) or haystack3MigrationConfig.csv (h3Name).
func readCSV(data []byte) (*File, error) {
	t, err := readTable(data)
	if err != nil {
		return nil, err
	}
	var f *File
	switch {
	case t.has("h3Name"):
		f = &File{Kind: "migrations"}
	case t.has("kind"):
		f = &File{Kind: "tags"}
	case t.has("tags"):
		f = &File{Kind: "groups"}
	default:
		return nil, errors.New("CSV is neither a tag, tag group nor migration table")
	}
	for i, row := range t.rows {
		top, rules := t.cells(row)
		if len(top) == 0 && len(rules) == 0 {
			continue
		}
		name := top["name"] + top["h3Name"]
		if name == "" {
			return nil, fmt.Errorf("row %d below the header has no name", i+1)
		}
		switch f.Kind {
		case "migrations":
			f.Migrations = append(f.Migrations, Migration{From: name, FromType: top["h3Type"], Query: top["h3Query"],
				To: splitTrim(top["h4Names"], "|"), ToType: top["h4Type"]})
		case "tags":
			f.Tags = append(f.Tags, Tag{Dict: Haystack, Name: name, Kind: top["kind"], SmartType: top["SmartType"],
				Units: top["units"], Implied: rules["impliedTagRules"], Validity: rules["Validity Rules"]})
		default:
			f.Groups = append(f.Groups, Group{Dict: Haystack, Name: name, Tags: strings.Fields(top["tags"]),
				Validity: rules["Validity Rules"]})
		}
	}
	return f, nil
}

// splitTrim splits s at sep, dropping blanks around and empty parts.
func splitTrim(s, sep string) []string {
	var out []string
	for _, p := range strings.Split(s, sep) {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
