package teachers

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Nodira Karimova":  "nodira-karimova",
		"  Kim  Min-jun  ": "kim-min-jun",
		"Élan O'Brien":     "lan-o-brien",
		"José—García":      "jos-garc-a",
		"C++ & Go tutor":   "c-go-tutor",
		"---":              "teacher",
		"":                 "teacher",
		"Аружан":           "teacher", // non-ASCII dropped
		"MiXeD CaSe 123":   "mixed-case-123",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
