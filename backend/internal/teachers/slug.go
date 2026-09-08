package teachers

import "strings"

// slugify turns a display name into a URL-safe slug: lower-case ASCII letters
// and digits, runs of anything else collapsed to a single hyphen, no leading or
// trailing hyphen. Non-ASCII letters are dropped (the demo catalog uses
// transliterated names already); if nothing survives, it falls back to
// "teacher" so the caller always has a base to de-duplicate.
func slugify(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	prevHyphen := false
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevHyphen = false
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
