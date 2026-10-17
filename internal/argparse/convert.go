package argparse

import "github.com/angeles725/niagara-go/internal/pycompat"

// intValue is the namespace value of type=int: an int, or a *big.Int beyond int's range.
func intValue(text string) (any, error) {
	n, err := pycompat.ParseBigInt(text)
	if err != nil {
		return nil, err
	}
	if n.IsInt64() && int64(int(n.Int64())) == n.Int64() {
		return int(n.Int64()), nil
	}
	return n, nil
}

func floatValue(text string) (any, error) { return pycompat.ParseFloat(text) }
