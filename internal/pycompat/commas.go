package pycompat

// Integer is the Go integer kinds Commas accepts, named types included.
type Integer interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr
}

// Commas is f"{n:,}" (format(n, ",")): the decimal digits of n grouped by three with
// commas, the sign kept ("-1,234,567").
func Commas[T Integer](n T) string {
	var v any = uint64(n)
	if n < 0 {
		v = int64(n)
	}
	s, _ := Format(v, ",") // "," never fails for an int
	return s
}
