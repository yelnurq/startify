package user

import (
	"regexp"
	"strings"
	"unicode"
)

var translit = map[rune]string{
	'а': "a", 'ә': "a", 'б': "b", 'в': "v", 'г': "g", 'ғ': "g", 'д': "d", 'е': "e", 'ё': "yo",
	'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'қ': "q", 'л': "l", 'м': "m", 'н': "n",
	'ң': "n", 'о': "o", 'ө': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u", 'ұ': "u",
	'ү': "u", 'ф': "f", 'х': "kh", 'һ': "h", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "shch",
	'ъ': "", 'ы': "y", 'і': "i", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
}

// SlugPattern is the shape of a public profile address.
var SlugPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$`)

// reservedSlugs could be mistaken for pages of the platform.
var reservedSlugs = map[string]bool{
	"admin": true, "api": true, "app": true, "auth": true, "login": true, "logout": true,
	"me": true, "new": true, "profile": true, "profiles": true, "qadam": true, "register": true,
	"root": true, "settings": true, "support": true, "system": true, "help": true, "edit": true,
}

// Slugify turns a name in Kazakh, Russian or English into an address part:
// "Айгерім Сәрсенова" -> "aygerim-sarsenova".
func Slugify(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case translit[r] != "" || r == 'ъ' || r == 'ь':
			b.WriteString(translit[r])
			dash = false
		case unicode.IsSpace(r) || r == '-' || r == '_' || r == '.' || r == '\'':
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	s := strings.Trim(b.String(), "-")
	if len(s) > 30 {
		s = strings.TrimRight(s[:30], "-")
	}
	if len(s) < 3 {
		s = "member" + s
	}
	return s
}

// slugAllowed reports whether a slug is well formed and not reserved.
func slugAllowed(s string) bool { return SlugPattern.MatchString(s) && !reservedSlugs[s] }
