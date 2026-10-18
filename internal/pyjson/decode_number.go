package pyjson

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
)

func isDigit(r rune) bool { return r >= '0' && r <= '9' }

func (p *jsonScanner) scanNumber(start int) (Value, int) {
	s, i := p.s, start
	if s[i] == '-' {
		i++
		if i >= len(s) {
			panic(stopIteration(start))
		}
	}
	switch {
	case s[i] >= '1' && s[i] <= '9':
		i++
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	case s[i] == '0':
		i++
	default:
		panic(stopIteration(start))
	}
	isFloat := false
	if i+1 < len(s) && s[i] == '.' && isDigit(s[i+1]) {
		isFloat = true
		i += 2
		for i < len(s) && isDigit(s[i]) {
			i++
		}
	}
	if i+1 < len(s) && (s[i] == 'e' || s[i] == 'E') {
		e := i
		i++
		if i+1 < len(s) && (s[i] == '-' || s[i] == '+') {
			i++
		}
		for i < len(s) && isDigit(s[i]) {
			i++
		}
		if isDigit(s[i-1]) {
			isFloat = true
		} else {
			i = e
		}
	}
	text := string(s[start:i])
	if isFloat {
		f, _ := strconv.ParseFloat(text, 64)
		return Value{Kind: Float, F: f}, i
	}
	if digits := len(strings.TrimPrefix(text, "-")); digits > IntDigitsLimit {
		panic(jsonError{&IntDigitsError{digits}})
	}
	n, _ := new(big.Int).SetString(text, 10)
	return Value{Kind: Int, I: n}, i
}

// IntDigitsLimit is CPython's default int_max_str_digits: json.loads refuses an int
// literal with more digits (a float literal has no limit).
const IntDigitsLimit = 4300

// IntDigitsError is the ValueError json.loads raises at the first int literal longer than
// IntDigitsLimit digits (the sign is not counted).
type IntDigitsError struct{ Digits int }

func (e *IntDigitsError) Error() string {
	return fmt.Sprintf("Exceeds the limit (%d digits) for integer string conversion: value has %d digits; "+
		"use sys.set_int_max_str_digits() to increase the limit", IntDigitsLimit, e.Digits)
}
