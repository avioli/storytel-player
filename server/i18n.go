package main

// Port of src/i18n: language negotiation plus the tray's translations.
// The renderer still loads its own bundles from /api/translations.

import (
	"embed"
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
)

//go:embed locales/*.json
var localeFS embed.FS

// Adding a language? Also update client/src/config/languages.ts.
var supportedLanguages = []string{"de", "en", "es", "fr", "it", "fi", "sv"}

const autoLanguage = "auto"
const fallbackLanguage = "en"

func localeJSON(code string) []byte {
	raw, _ := localeFS.ReadFile("locales/" + code + ".json")
	return raw
}

func isSupportedLanguage(code string) bool {
	for _, c := range supportedLanguages {
		if c == strings.ToLower(code) {
			return true
		}
	}
	return false
}

// matchLanguage accepts region variants and odd casing ("sv-FI", "SV", "sv_FI").
func matchLanguage(lang string) string {
	requested := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(lang)), "_", "-")
	base := strings.Split(requested, "-")[0]
	for _, c := range supportedLanguages {
		if c == requested || c == base {
			return c
		}
	}
	return ""
}

// negotiateLanguage: ['sv-FI', 'en-FI'] -> 'sv'; the region never decides.
func negotiateLanguage(preferred []string) string {
	for _, raw := range preferred {
		parts := strings.FieldsFunc(strings.ToLower(strings.TrimSpace(raw)), func(r rune) bool { return r == '-' || r == '_' })
		for i := len(parts); i > 0; i-- {
			if c := strings.Join(parts[:i], "-"); isSupportedLanguage(c) {
				return c
			}
		}
	}
	return fallbackLanguage
}

var quoted = regexp.MustCompile(`"?([A-Za-z]{2,3}(?:[-_][A-Za-z0-9]+)*)"?`)

// systemLanguages returns the OS languages, most preferred first.
func systemLanguages() []string {
	var langs []string
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("defaults", "read", "-g", "AppleLanguages").Output(); err == nil {
			for _, m := range quoted.FindAllStringSubmatch(string(out), -1) {
				langs = append(langs, m[1])
			}
		}
	}
	for _, env := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := strings.Split(os.Getenv(env), ".")[0]; v != "" && v != "C" && v != "POSIX" {
			langs = append(langs, strings.ReplaceAll(v, "_", "-"))
		}
	}
	return langs
}

// systemRegion is the OS country code ("FI"), i.e. where the user is.
func systemRegion() any {
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("defaults", "read", "-g", "AppleLocale").Output(); err == nil {
			loc := strings.Split(strings.TrimSpace(string(out)), "@")[0]
			if parts := strings.Split(loc, "_"); len(parts) > 1 {
				return strings.ToUpper(parts[len(parts)-1])
			}
		}
	}
	return nil
}

type I18n struct {
	mu           sync.Mutex
	store        *Store
	current      string
	translations map[string]any
	system       []string
	region       any
}

func NewI18n(store *Store) *I18n {
	i := &I18n{store: store, system: systemLanguages(), region: systemRegion()}
	i.Detect()
	return i
}

func (i *I18n) Mode() string {
	if saved, ok := i.store.Get("appLanguage").(string); ok && saved != autoLanguage && isSupportedLanguage(saved) {
		return strings.ToLower(saved)
	}
	return autoLanguage
}

func (i *I18n) systemResolved() string {
	return negotiateLanguage(append(append([]string{}, i.system...), os.Getenv("APP_LOCALE")))
}

func (i *I18n) Detect() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.current = i.Mode()
	if i.current == autoLanguage {
		i.current = i.systemResolved()
	}
	i.translations = map[string]any{}
	_ = json.Unmarshal(localeJSON(i.current), &i.translations)
}

func (i *I18n) Language() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.current
}

func (i *I18n) State() map[string]any {
	return map[string]any{
		"mode":            i.Mode(),
		"resolved":        i.Language(),
		"systemLanguages": i.system,
		"systemResolved":  i.systemResolved(),
		"region":          i.region,
	}
}

// T looks up a dot-path key, falling back to the key itself.
func (i *I18n) T(key string) string {
	i.mu.Lock()
	defer i.mu.Unlock()
	var cur any = i.translations
	for _, part := range strings.Split(key, ".") {
		cur = obj(cur)[part]
	}
	if s, ok := cur.(string); ok {
		return s
	}
	return key
}
