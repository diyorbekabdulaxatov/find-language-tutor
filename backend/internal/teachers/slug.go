package teachers

import (
	"strings"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

// cyrillic maps the Russian alphabet plus the four Uzbek-Cyrillic letters to
// the Latin spelling a person would use themselves (Uzbek Latin conventions
// for the shared letters: ж→j, х→x, ц→ts, ч→ch, ш→sh, ю→yu, я→ya). It runs
// before the ASCII pass, so "Нодира Каримова" → nodira-karimova instead of
// the anonymous "teacher-2".
var cyrillic = map[rune]string{
	'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo", 'ж': "j",
	'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m", 'н': "n", 'о': "o",
	'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ф': "f", 'х': "x", 'ц': "ts",
	'ч': "ch", 'ш': "sh", 'щ': "sch", 'ъ': "", 'ы': "i", 'ь': "", 'э': "e", 'ю': "yu",
	'я': "ya",
	// Uzbek Cyrillic
	'ў': "o", 'қ': "q", 'ғ': "g", 'ҳ': "h",
}

func transliterate(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if tr, ok := cyrillic[r]; ok {
			b.WriteString(tr)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stripMarks removes combining accents so "é" → "e", "ö" → "o" survive the
// ASCII pass instead of being dropped.
var stripMarks = transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC)

// slugify turns a display name into a URL-safe slug: lower-case ASCII letters
// and digits, runs of anything else collapsed to a single hyphen, no leading or
// trailing hyphen. Cyrillic is transliterated and Latin accents stripped first;
// if nothing survives, it falls back to "teacher" so the caller always has a
// base to de-duplicate.
func slugify(s string) string {
	s = transliterate(strings.ToLower(s))
	// Accent stripping runs *after* transliteration: NFD would otherwise
	// decompose ў / й / ё into a base letter plus a mark and lose them.
	if t, _, err := transform.String(stripMarks, s); err == nil {
		s = t
	}
	var b strings.Builder
	b.Grow(len(s))
	prevHyphen := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
		case r == '\'' || r == 'ʼ' || r == '’':
			// The apostrophe in Uzbek Latin (o', g') is not a word break.
			continue
		default:
			if !prevHyphen {
				b.WriteByte('-')
				prevHyphen = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "teacher"
	}
	return out
}
