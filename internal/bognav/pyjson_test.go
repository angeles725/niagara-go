package bognav

import "testing"

// A *string or []row nested in a []any is converted at any depth, like the encoder it replaced.
func TestJSONValueNested(t *testing.T) {
	s := "x"
	got, err := dumps([]any{&s, (*string)(nil), []any{[]row{{{"k", &s}}}}}, -1)
	if err != nil {
		t.Fatal(err)
	}
	if want := `["x", null, [[{"k": "x"}]]]`; got != want {
		t.Fatalf("dumps = %s, want %s", got, want)
	}
}
