package argparse

import "strings"

// formatUsage is HelpFormatter._format_usage.
func (f *formatter) formatUsage(usage string, actions []*Action, groups []*MutexGroup, prefix string) string {
	switch {
	case usage != "":
		expanded, err := pyPercent(usage, map[string]any{"prog": f.prog})
		if err != nil {
			panic("argparse: badly formed usage: " + err.Error())
		}
		usage = expanded
	case len(actions) == 0:
		usage = f.prog
	default:
		prog := f.prog
		parts, posStart := f.usageParts(actions, groups)
		words := []string{prog}
		for _, part := range parts {
			if part != "" {
				words = append(words, part)
			}
		}
		usage = strings.Join(words, " ")
		textWidth := f.width - f.currentIndent
		if runeLen(prefix)+runeLen(usage) > textWidth {
			usage = strings.Join(f.wrapUsage(prog, prefix, parts[:posStart], parts[posStart:], textWidth), "\n")
		}
	}
	return prefix + usage + "\n\n"
}

// wrapUsage splits a long usage: options after prog, then positionals on their own lines.
func (f *formatter) wrapUsage(prog, prefix string, opt, pos []string, textWidth int) []string {
	getLines := func(parts []string, indent string, withPrefix bool) []string {
		var lines, line []string
		indentLen := len(indent)
		lineLen := indentLen - 1
		if withPrefix {
			lineLen = runeLen(prefix) - 1
		}
		for _, part := range parts {
			n := runeLen(part)
			if lineLen+1+n > textWidth && len(line) > 0 {
				lines = append(lines, indent+strings.Join(line, " "))
				line = nil
				lineLen = indentLen - 1
			}
			line = append(line, part)
			lineLen += n + 1
		}
		if len(line) > 0 {
			lines = append(lines, indent+strings.Join(line, " "))
		}
		if withPrefix {
			lines[0] = lines[0][indentLen:]
		}
		return lines
	}
	progLen := runeLen(prog)
	if float64(runeLen(prefix)+progLen) <= 0.75*float64(textWidth) {
		indent := spaces(runeLen(prefix) + progLen + 1)
		switch {
		case len(opt) > 0:
			lines := getLines(append([]string{prog}, opt...), indent, true)
			return append(lines, getLines(pos, indent, false)...)
		case len(pos) > 0:
			return getLines(append([]string{prog}, pos...), indent, true)
		}
		return []string{prog}
	}
	indent := spaces(runeLen(prefix))
	lines := getLines(append(append([]string{}, opt...), pos...), indent, false)
	if len(lines) > 1 {
		lines = append(getLines(opt, indent, false), getLines(pos, indent, false)...)
	}
	return append([]string{prog}, lines...)
}

type usageGroup struct {
	required bool
	actions  []*Action
}

// usageParts is _get_actions_usage_parts: one part per argument, mutually exclusive
// groups bracketed together; posStart is the index of the first positional part.
func (f *formatter) usageParts(actions []*Action, groups []*MutexGroup) ([]string, int) {
	var acts []*Action
	present := map[*Action]bool{}
	for _, a := range actions {
		if a.Help != Suppress {
			acts = append(acts, a)
			present[a] = true
		}
	}
	groupOf := map[*Action]*MutexGroup{}
	for _, g := range groups {
		for _, a := range g.actions {
			if present[a] {
				groupOf[a] = g
			}
		}
	}
	pop := func(a *Action) bool {
		ok := present[a]
		delete(present, a)
		return ok
	}
	var positionals, optionals []usageGroup
	for _, a := range acts {
		if a.isOptional() {
			continue
		}
		g := groupOf[a]
		pop(a)
		if g == nil {
			positionals = append(positionals, usageGroup{false, []*Action{a}})
			continue
		}
		var ga []*Action
		for _, a2 := range g.actions {
			if a2.isOptional() && pop(a2) {
				ga = append(ga, a2)
			}
		}
		positionals = append(positionals, usageGroup{g.Required, append(ga, a)})
	}
	for _, a := range acts {
		if !a.isOptional() || !present[a] {
			continue
		}
		g := groupOf[a]
		pop(a)
		if g == nil {
			optionals = append(optionals, usageGroup{false, []*Action{a}})
			continue
		}
		ga := []*Action{a}
		for _, a2 := range g.actions {
			if a2.isOptional() && pop(a2) {
				ga = append(ga, a2)
			}
		}
		optionals = append(optionals, usageGroup{g.Required, ga})
	}

	var parts []string
	posStart := -1
	for i, ug := range append(optionals, positionals...) {
		start := len(parts)
		if i == len(optionals) {
			posStart = start
		}
		inGroup := len(ug.actions) > 1
		for _, a := range ug.actions {
			var part string
			if !a.isOptional() {
				part = f.formatArgs(a, a.Dest)
				if inGroup && strings.HasPrefix(part, "[") && strings.HasSuffix(part, "]") {
					part = part[1 : len(part)-1]
				}
			} else {
				part = a.OptionStrings[0]
				if !a.nargsZero {
					part += " " + f.formatArgs(a, strings.ToUpper(a.Dest))
				}
				if !(a.Required || ug.required || inGroup) {
					part = "[" + part + "]"
				}
			}
			parts = append(parts, part)
		}
		if inGroup {
			open, closing := "[", "]"
			if ug.required {
				open, closing = "(", ")"
			}
			parts[start] = open + parts[start]
			for k := start; k < len(parts)-1; k++ {
				parts[k] += " |"
			}
			parts[len(parts)-1] += closing
		}
	}
	if posStart < 0 {
		posStart = len(parts)
	}
	return parts, posStart
}
