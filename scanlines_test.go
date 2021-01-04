package stripeutil

import (
	"strings"
	"testing"
)

func Test_scanlines(t *testing.T) {
	s := `line_1
line_2

# commet
line_3




line_4`

	expected := []string{
		"line_1",
		"line_2",
		"line_3",
		"line_4",
	}

	actual := make([]string, 0, len(expected))

	err := scanlines(strings.NewReader(s), func(line string) {
		actual = append(actual, line)
	})

	if err != nil {
		t.Fatal(err)
	}

	if len(expected) != len(actual) {
		t.Fatalf("unexpected number of lines scanned, expected=%d, got=%d\n", len(expected), len(actual))
	}

	for i, s := range expected {
		if actual[i] != s {
			t.Errorf("unexpected string in scanned lines, expected=%q, got=%q\n", s, actual[i])
		}
	}
}
