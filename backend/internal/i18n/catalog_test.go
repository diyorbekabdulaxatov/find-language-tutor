package i18n

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// userFacingCalls maps a callee to the argument positions that carry a
// user-facing message. -1 means "every string-literal argument that looks
// like prose" (used for the per-module invalid(...) helpers, whose first arg
// is sometimes a code).
var userFacingCalls = map[string][]int{
	"web.BadRequest":      {1},
	"web.Unauthorized":    {1},
	"web.Forbidden":       {1},
	"web.NotFound":        {1},
	"web.PayloadTooLarge": {1},
	"web.BadRequestf":     {1},
	"web.Forbiddenf":      {1},
	"web.NotFoundf":       {1},
	"web.WriteError":      {3},
	"web.WriteErrorf":     {3},
	"invalid":             {-1},
	"i18n.Message":        {0},
	"i18n.T":              {1},
	"i18n.Tf":             {1},
	"T":                   {1},
	"Tf":                  {1},
	"Message":             {0},
}

// sourceStrings walks every non-test Go file under root and returns the set
// of user-facing message literals it finds.
func sourceStrings(t *testing.T, root string) map[string][]string {
	t.Helper()
	found := map[string][]string{} // message -> where
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "sqlc" || d.Name() == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			positions, ok := userFacingCalls[calleeName(call)]
			if !ok {
				return true
			}
			for i, arg := range call.Args {
				lit, ok := arg.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				if !wanted(positions, i) {
					continue
				}
				s, err := strconv.Unquote(lit.Value)
				if err != nil || !looksLikeProse(s) {
					continue
				}
				found[s] = append(found[s], fset.Position(lit.Pos()).String())
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return found
}

func wanted(positions []int, i int) bool {
	for _, p := range positions {
		if p == -1 || p == i {
			return true
		}
	}
	return false
}

// looksLikeProse separates messages from codes/keys: a message has a space
// or ends with punctuation.
func looksLikeProse(s string) bool {
	return strings.Contains(s, " ") || strings.HasSuffix(s, ".") || strings.HasSuffix(s, "!")
}

func calleeName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if x, ok := fn.X.(*ast.Ident); ok {
			return x.Name + "." + fn.Sel.Name
		}
	}
	return ""
}

// TestCatalogsCoverEverySourceString is the guard: every user-facing literal
// in the API must have a translation in every non-English locale. Run with
// I18N_DUMP=1 to print the full source set as JSON (handy when adding a
// batch of messages).
func TestCatalogsCoverEverySourceString(t *testing.T) {
	found := sourceStrings(t, "../..")
	if len(found) < 100 {
		t.Fatalf("only %d user-facing strings found — extractor is probably broken", len(found))
	}

	if os.Getenv("I18N_DUMP") != "" {
		keys := make([]string, 0, len(found))
		for k := range found {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		out, _ := json.MarshalIndent(keys, "", "  ")
		t.Logf("SOURCE STRINGS (%d):\n%s", len(keys), out)
	}

	for _, loc := range Locales {
		if loc == Default {
			continue
		}
		cat := catalogs[loc]
		var missing []string
		for msg := range found {
			if tr, ok := cat[msg]; !ok || strings.TrimSpace(tr) == "" {
				missing = append(missing, msg)
			}
		}
		sort.Strings(missing)
		for _, m := range missing {
			t.Errorf("%s: no translation for %q (%s)", loc, m, found[m][0])
		}
		var stale []string
		for msg := range cat {
			if _, ok := found[msg]; !ok {
				stale = append(stale, msg)
			}
		}
		if len(stale) > 0 {
			sort.Strings(stale)
			t.Logf("%s: %d catalog entries no longer appear in the source (harmless, but prune when convenient): %q", loc, len(stale), stale)
		}
	}
}

func TestNegotiate(t *testing.T) {
	cases := map[string]string{
		"":                        "en",
		"ru":                      "ru",
		"ru-RU,ru;q=0.9,en;q=0.8": "ru",
		"uz-Latn-UZ":              "uz",
		"en-GB,en;q=0.9":          "en",
		"fr-FR,fr;q=0.9,de;q=0.8": "en",
		"fr, uz":                  "uz",
		"UZ":                      "uz",
	}
	for in, want := range cases {
		if got := Negotiate(in); got != want {
			t.Errorf("Negotiate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMsgLocalizesFormatBeforeSprintf(t *testing.T) {
	m := Message("Question %d has no prompt.", 3)
	if m.String() != "Question 3 has no prompt." {
		t.Errorf("String() = %q", m.String())
	}
	if got := m.Localize("ru"); got == m.String() || !strings.Contains(got, "3") {
		t.Errorf("Localize(ru) = %q, want a Russian sentence containing 3", got)
	}
	if got := m.Localize("xx"); got != m.String() {
		t.Errorf("unknown locale should fall back to English, got %q", got)
	}
}

var verbRE = regexp.MustCompile(`%[+#\- 0-9.]*[a-zA-Z%]`)

// TestCatalogsKeepFormatVerbs: a translation must carry the same verbs in
// the same order as its source, or Sprintf would print %!s(MISSING) or bind
// arguments to the wrong slots. Reordering is only allowed with explicit
// argument indexes, which we don't use.
func TestCatalogsKeepFormatVerbs(t *testing.T) {
	for loc, cat := range catalogs {
		for src, tr := range cat {
			want := verbRE.FindAllString(src, -1)
			got := verbRE.FindAllString(tr, -1)
			if strings.Join(want, ",") != strings.Join(got, ",") {
				t.Errorf("%s: %q → %q: verbs %v vs %v", loc, src, tr, want, got)
			}
		}
	}
}
