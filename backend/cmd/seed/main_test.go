package main

import (
	"testing"

	"github.com/yelnurq/startify/backend/internal/platform/config"
)

// A password for the seed in the environment must not stop the seed: the
// configuration refuses unknown QADAM_* variables.
func TestSeedVariablesAreNotConfigurationKeys(t *testing.T) {
	env := []string{
		"QADAM_DATABASE_URL=postgres://qadam_worker:secret@localhost:5432/qadam",
		"QADAM_AUTH_SECRET_KEY=test-secret-key-with-enough-length-0123456789",
		"QADAM_S3_ENDPOINT=localhost:9000",
		"QADAM_S3_ACCESS_KEY=qadam",
		"QADAM_S3_SECRET_KEY=qadam-secret",
		envDemoPassword + "=Presentation-2026",
		envAdminPassword + "=Admin-Password-2026",
	}
	if _, err := config.FromEnviron(env); err != nil {
		t.Fatal(err)
	}
}

func TestSplitList(t *testing.T) {
	got := splitList(" KazTBU.edu.kz, ,student.kaztbu.edu.kz ")
	if len(got) != 2 || got[0] != "kaztbu.edu.kz" || got[1] != "student.kaztbu.edu.kz" {
		t.Fatalf("splitList: %q", got)
	}
	if splitList("") != nil {
		t.Fatal("an empty list is not nil")
	}
}
