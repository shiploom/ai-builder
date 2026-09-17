package difflib

import (
	"reflect"
	"testing"
)

func TestUnifiedDiffCorpus(t *testing.T) {
	bigA := make([]string, 0, 301)
	bigB := make([]string, 0, 301)
	for i := 0; i < 300; i++ {
		bigA = append(bigA, "common "+itoa(i%5))
		bigB = append(bigB, "common "+itoa(i%5))
	}
	bigA = append(bigA, "tail-a")
	bigB = append(bigB, "tail-b")

	cases := []struct {
		name string
		a, b []string
		want []string
	}{
		{"simple", []string{"hello"}, []string{"world"},
			[]string{"--- before\n", "+++ after\n", "@@ -1 +1 @@\n", "-hello", "+world"}},
		{"multi",
			[]string{"a", "b", "c", "d", "e", "f", "g"},
			[]string{"a", "B", "c", "d", "E", "f", "g"},
			[]string{"--- before\n", "+++ after\n", "@@ -1,7 +1,7 @@\n",
				" a", "-b", "+B", " c", " d", "-e", "+E", " f", " g"}},
		{"insert",
			[]string{"a", "d"}, []string{"a", "b", "c", "d"},
			[]string{"--- before\n", "+++ after\n", "@@ -1,2 +1,4 @@\n",
				" a", "+b", "+c", " d"}},
		{"delete",
			[]string{"a", "b", "c", "d"}, []string{"a", "d"},
			[]string{"--- before\n", "+++ after\n", "@@ -1,4 +1,2 @@\n",
				" a", "-b", "-c", " d"}},
		{"empty-old",
			[]string{}, []string{"x", "y"},
			[]string{"--- before\n", "+++ after\n", "@@ -0,0 +1,2 @@\n", "+x", "+y"}},
		{"empty-new",
			[]string{"x"}, []string{},
			[]string{"--- before\n", "+++ after\n", "@@ -1 +0,0 @@\n", "-x"}},
		{"blank-lines",
			[]string{"a", "", "b"}, []string{"a", "", "c"},
			[]string{"--- before\n", "+++ after\n", "@@ -1,3 +1,3 @@\n",
				" a", " ", "-b", "+c"}},
		{"big-repetitive", bigA, bigB,
			[]string{"--- before\n", "+++ after\n", "@@ -298,4 +298,4 @@\n",
				" common 2", " common 3", " common 4", "-tail-a", "+tail-b"}},
		{"unicode",
			[]string{"héllo", "wörld"}, []string{"héllo!", "wörld"},
			[]string{"--- before\n", "+++ after\n", "@@ -1,2 +1,2 @@\n",
				"-héllo", "+héllo!", " wörld"}},
		{"trailing-space",
			[]string{"abc ", "def"}, []string{"abc", "def"},
			[]string{"--- before\n", "+++ after\n", "@@ -1,2 +1,2 @@\n",
				"-abc ", "+abc", " def"}},
	}
	for _, tc := range cases {
		if got := UnifiedDiff(tc.a, tc.b); !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s:\n got: %q\nwant: %q", tc.name, got, tc.want)
		}
	}
	if out := UnifiedDiff([]string{"a", "b"}, []string{"a", "b"}); len(out) != 0 {
		t.Fatalf("identical inputs must yield no output, got %q", out)
	}
	var _ = reflect.DeepEqual
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
