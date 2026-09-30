package auth

import "testing"

func TestParseGroups(t *testing.T) {
	cases := []struct {
		name string
		in   any
		want []string
	}{
		{"missing", nil, nil},
		{"array", []any{"admin", "dev"}, []string{"admin", "dev"}},
		{"single string", "admin", []string{"admin"}},
		{"empty string", "", nil},
		{"drops non-strings", []any{"admin", 42, nil}, []string{"admin"}},
		{"wrong type", 42, nil},
	}
	for _, c := range cases {
		got := parseGroups(c.in)
		if len(got) != len(c.want) {
			t.Fatalf("%s: got %v want %v", c.name, got, c.want)
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Fatalf("%s: got %v want %v", c.name, got, c.want)
			}
		}
	}
}
