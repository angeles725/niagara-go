package argparse

import "strings"

// Option-string classification: _parse_optional and _get_option_tuples.

// optTuple is one interpretation of an option string: (action, option_string, sep,
// explicit_arg); hasSep and hasArg false stand for None.
type optTuple struct {
	action *Action
	option string
	sep    string
	hasSep bool
	arg    string
	hasArg bool
}

// classifyArgs finds the option strings (option_string_indices) and builds
// arg_strings_pattern: 'O' for an option, 'A' for an argument, '-' for the first "--"
// after which every string is an argument.
func (p *Parser) classifyArgs(argStrings []string) (map[int][]optTuple, string) {
	optionIdx := map[int][]optTuple{}
	var pat strings.Builder
	for i := 0; i < len(argStrings); i++ {
		if argStrings[i] == "--" {
			pat.WriteByte('-')
			pat.WriteString(strings.Repeat("A", len(argStrings)-i-1))
			break
		}
		if t := p.parseOptional(argStrings[i]); t != nil {
			optionIdx[i] = t
			pat.WriteByte('O')
		} else {
			pat.WriteByte('A')
		}
	}
	return optionIdx, pat.String()
}

// parseOptional is _parse_optional: nil when the string is a positional argument.
func (p *Parser) parseOptional(s string) []optTuple {
	if s == "" || s[0] != '-' {
		return nil
	}
	if a, ok := p.optIndex[s]; ok {
		return []optTuple{{action: a, option: s}}
	}
	if s == "-" {
		return nil
	}
	if opt, arg, found := strings.Cut(s, "="); found {
		if a, ok := p.optIndex[opt]; ok {
			return []optTuple{{action: a, option: opt, sep: "=", hasSep: true, arg: arg, hasArg: true}}
		}
	}
	if t := p.optionTuples(s); len(t) > 0 {
		return t
	}
	if negativeNumber(s) && !p.negOptions {
		return nil
	}
	if strings.Contains(s, " ") {
		return nil
	}
	return []optTuple{{option: s}}
}

// optionTuples is _get_option_tuples: the abbreviations and short-option-with-value
// readings of s.
func (p *Parser) optionTuples(s string) []optTuple {
	var out []optTuple
	prefix, arg, found := strings.Cut(s, "=")
	if s[1] == '-' {
		if p.cfg.NoAbbrev {
			return nil
		}
		for _, o := range p.optOrder {
			if strings.HasPrefix(o, prefix) {
				out = append(out, optTuple{action: p.optIndex[o], option: o, sep: "=", hasSep: found, arg: arg, hasArg: found})
			}
		}
		return out
	}
	first, rest := splitFirstRune(s[1:])
	short := "-" + first
	for _, o := range p.optOrder {
		if o == short {
			out = append(out, optTuple{action: p.optIndex[o], option: o, hasSep: true, arg: rest, hasArg: true})
		} else if !p.cfg.NoAbbrev && strings.HasPrefix(o, prefix) {
			out = append(out, optTuple{action: p.optIndex[o], option: o, sep: "=", hasSep: found, arg: arg, hasArg: found})
		}
	}
	return out
}

func splitFirstRune(s string) (string, string) {
	for i := range s {
		if i > 0 {
			return s[:i], s[i:]
		}
	}
	return s, ""
}
