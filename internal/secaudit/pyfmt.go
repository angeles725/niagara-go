package secaudit

// OSError.strerror of a Go error, which internal/pycompat does not provide for an error value.

import (
	"errors"
	"io/fs"
	"syscall"
	"unicode"
	"unicode/utf8"
)

// strerror is the C strerror text Python shows for an OS error ("Permission denied").
func strerror(err error) string {
	var errno syscall.Errno
	if errors.As(err, &errno) {
		msg := errno.Error()
		if r, size := utf8.DecodeRuneInString(msg); size > 0 {
			return string(unicode.ToUpper(r)) + msg[size:]
		}
		return msg
	}
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err.Error()
	}
	return err.Error()
}
