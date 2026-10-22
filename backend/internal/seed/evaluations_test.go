package seed

import "testing"

func TestContentLocale(t *testing.T) {
	cases := map[string]string{
		"Фермеры Костанайской области теряют урожай. Команда: Нұрлан Жұмабеков, Дана Ахметова.": "ru",
		"Ауылдағы шебер әйелдер киіз бұйымдарын тек жәрмеңкелерде сатады.":                      "kk",
		"Applicants ask the same questions in Kazakh and Russian.":                              "en",
	}
	for text, want := range cases {
		if got := contentLocale(text); got != want {
			t.Errorf("%q: %s, want %s", text, got, want)
		}
	}
}
