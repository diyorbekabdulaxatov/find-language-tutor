// Package i18n localises the strings the API hands to people: error
// messages and transactional email. It is gettext-shaped on purpose — the
// English text in the Go source *is* the key, and catalog/<locale>.json maps
// it to a translation — so call sites keep reading as plain English and a
// missing translation degrades to English rather than to a key.
//
// A test in this package walks the source tree and fails when a user-facing
// literal has no entry in every non-English catalog, which is what keeps the
// catalogs honest as messages are added.
package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

//go:embed catalog/*.json
var catalogFS embed.FS

const (
	Default = "en"
)

// Locales is every UI locale the platform speaks, English first.
var Locales = []string{"en", "ru", "uz"}

// catalogs holds source → translation per non-English locale.
var catalogs = map[string]map[string]string{}

func init() {
	for _, loc := range Locales {
		if loc == Default {
			continue
		}
		raw, err := catalogFS.ReadFile("catalog/" + loc + ".json")
		if err != nil {
			panic(fmt.Sprintf("i18n: catalog for %q missing: %v", loc, err))
		}
		m := map[string]string{}
		if err := json.Unmarshal(raw, &m); err != nil {
			panic(fmt.Sprintf("i18n: catalog for %q is not valid JSON: %v", loc, err))
		}
		catalogs[loc] = m
	}
}

// Supported reports whether loc is a locale we have (or is English).
func Supported(loc string) bool {
	for _, l := range Locales {
		if l == loc {
			return true
		}
	}
	return false
}

// Negotiate picks a locale from an Accept-Language header: the first listed
// language whose primary subtag we support, else the default. Quality values
// are ignored — browsers already list in preference order, and the frontend
// sends a single explicit tag.
func Negotiate(acceptLanguage string) string {
	for _, part := range strings.Split(acceptLanguage, ",") {
		tag := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		primary := strings.ToLower(strings.SplitN(tag, "-", 2)[0])
		if primary != "" && Supported(primary) {
			return primary
		}
	}
	return Default
}

// T translates msg into loc, or returns msg when there is no entry.
func T(loc, msg string) string {
	if m, ok := catalogs[loc]; ok {
		if tr, ok := m[msg]; ok && tr != "" {
			return tr
		}
	}
	return msg
}

// Tf translates a format string, then applies the arguments.
func Tf(loc, format string, args ...any) string {
	if len(args) == 0 {
		return T(loc, format)
	}
	return fmt.Sprintf(T(loc, format), args...)
}

// Msg is a message with its arguments kept apart, so it can be rendered in
// English for logs (String) or in a locale for people (Localize). Module
// ValidationError types embed it.
type Msg struct {
	Format string
	Args   []any
}

func Message(format string, args ...any) Msg { return Msg{Format: format, Args: args} }

func (m Msg) String() string { return fmt.Sprintf(m.Format, m.Args...) }

func (m Msg) Localize(loc string) string { return Tf(loc, m.Format, m.Args...) }

// Localizer is anything that can say itself in a locale — the module
// ValidationError types, via the embedded Msg.
type Localizer interface {
	Localize(loc string) string
}

// --- context plumbing ---

type ctxKey struct{}

// WithLocale stores the request's locale so services can reach it (mostly to
// pick a language for something addressed to the caller).
func WithLocale(ctx context.Context, loc string) context.Context {
	return context.WithValue(ctx, ctxKey{}, loc)
}

// FromContext returns the locale stored by WithLocale, or the default.
func FromContext(ctx context.Context) string {
	if loc, ok := ctx.Value(ctxKey{}).(string); ok && Supported(loc) {
		return loc
	}
	return Default
}

// --- dates ---

var (
	monthsShort = map[string][]string{
		"ru": {"янв", "фев", "мар", "апр", "мая", "июн", "июл", "авг", "сен", "окт", "ноя", "дек"},
		"uz": {"yan", "fev", "mar", "apr", "may", "iyn", "iyl", "avg", "sen", "okt", "noy", "dek"},
	}
	weekdaysShort = map[string][]string{
		"ru": {"вс", "пн", "вт", "ср", "чт", "пт", "сб"},
		"uz": {"yak", "dush", "sesh", "chor", "pay", "jum", "shan"},
	}
)

// FormatDateTime renders an instant for email: "Mon, 02 Jan 2006 15:04 MST"
// in English, with the weekday and month in the locale otherwise. The zone
// abbreviation stays as Go gives it — it is the same across languages.
func FormatDateTime(loc string, t time.Time) string {
	months, ok := monthsShort[loc]
	if !ok {
		return t.Format("Mon, 02 Jan 2006 15:04 MST")
	}
	return fmt.Sprintf("%s, %02d %s %d %s",
		weekdaysShort[loc][t.Weekday()], t.Day(), months[t.Month()-1], t.Year(), t.Format("15:04 MST"))
}
