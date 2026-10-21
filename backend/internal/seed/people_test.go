package seed

import (
	"net/mail"
	"testing"

	"github.com/yelnurq/startify/backend/internal/user"
)

func TestDemoPeopleAreConsistent(t *testing.T) {
	groups := map[string]bool{}
	for _, f := range KazUTBStructure {
		for _, d := range f.Departments {
			for _, g := range d.Groups {
				groups[g.Code] = true
			}
		}
	}
	names, emails, slugs := map[string]bool{}, map[string]bool{}, map[string]bool{}
	roles := map[string]int{}
	for _, m := range AllKazUTBMembers() {
		if names[m.FullName] || emails[m.Email] || slugs[m.Slug] {
			t.Errorf("duplicate person %s %s %s", m.FullName, m.Email, m.Slug)
		}
		names[m.FullName], emails[m.Email], slugs[m.Slug] = true, true, true
		if a, err := mail.ParseAddress(m.Email); err != nil || a.Address != m.Email {
			t.Errorf("bad e-mail %q", m.Email)
		}
		if !user.SlugPattern.MatchString(m.Slug) {
			t.Errorf("bad slug %q", m.Slug)
		}
		if m.Group != "" && !groups[m.Group] {
			t.Errorf("%s: unknown group %s", m.Email, m.Group)
		}
		for _, r := range m.Roles {
			roles[r]++
		}
	}
	// The demo needs a realistic university: ~60 students, 8 mentors, 6 jurors.
	for role, want := range map[string]int{"student": 60, "mentor": 8, "jury": 6, "innovation_manager": 2, "leadership": 1, "tenant_admin": 1} {
		if roles[role] != want {
			t.Errorf("%s: %d, want %d", role, roles[role], want)
		}
	}
	if len(KazUTBStructure) != 4 {
		t.Errorf("faculties: %d", len(KazUTBStructure))
	}
}
