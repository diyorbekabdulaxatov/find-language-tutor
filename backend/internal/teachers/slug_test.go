package teachers

import "testing"

func TestSlugify(t *testing.T) {
	cases := map[string]string{
		"Nodira Karimova":  "nodira-karimova",
		"  Kim  Min-jun  ": "kim-min-jun",
		"C++ & Go tutor":   "c-go-tutor",
		"---":              "teacher",
		"":                 "teacher",
		"MiXeD CaSe 123":   "mixed-case-123",
		// Latin accents survive instead of being dropped.
		"Élan O'Brien": "elan-obrien",
		"José—García":  "jose-garcia",
		// Uzbek Latin apostrophes are not word breaks.
		"G'ulom O'rinov": "gulom-orinov",
		// Cyrillic transliterates — Russian and the Uzbek-specific letters.
		"Нодира Каримова": "nodira-karimova",
		"Аружан":          "arujan",
		"Ғулом Ўринов":    "gulom-orinov",
		"Ҳусан Қодиров":   "husan-qodirov",
		"Шухрат Юсупов":   "shuxrat-yusupov",
		"Щукин Ъ":         "schukin",
	}
	for in, want := range cases {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}
