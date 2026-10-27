package user

import "testing"

func TestSlugify(t *testing.T) {
	for name, want := range map[string]string{
		"Айгерім Сәрсенова":     "aygerim-sarsenova",
		"Нұрлан Жұмабеков":      "nurlan-zhumabekov",
		"Гүлнар Қасымова":       "gulnar-qasymova",
		"Щукин Юрий Эдуардович": "shchukin-yuriy-eduardovich",
		"Ольга  О'Нил":          "olga-o-nil",
		"Timur Kim":             "timur-kim",
		"  Ким-Ли  ":            "kim-li",
		"Ю":                     "memberyu",
		"Әбілқайыр Мұхаммедқалиұлы Сейітжапаров": "abilqayyr-mukhammedqaliuly-sey",
		"!!!": "member",
	} {
		if got := Slugify(name); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", name, got, want)
		}
		if got := Slugify(name); !SlugPattern.MatchString(got) {
			t.Errorf("Slugify(%q) = %q does not match the pattern", name, got)
		}
	}
	for _, s := range []string{"admin", "me", "settings"} {
		if slugAllowed(s) {
			t.Errorf("%q must be reserved", s)
		}
	}
	if slugAllowed("-bad") || slugAllowed("Up") || !slugAllowed("aru-bekova") {
		t.Error("pattern check")
	}
}
