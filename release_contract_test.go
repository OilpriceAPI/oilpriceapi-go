package oilpriceapi

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestReleaseMetadataMatchesSDKVersion(t *testing.T) {
	changelog, err := os.ReadFile("CHANGELOG.md")
	if err != nil {
		t.Fatal(err)
	}

	firstRelease := regexp.MustCompile(`(?m)^## \[([^]]+)\]`).FindStringSubmatch(string(changelog))
	if len(firstRelease) != 2 {
		t.Fatal("CHANGELOG.md has no release heading")
	}
	if firstRelease[1] != Version {
		t.Fatalf("latest changelog version %q does not match SDK Version %q", firstRelease[1], Version)
	}
}

func TestWorkflowUsesCurrentNode24Actions(t *testing.T) {
	workflow, err := os.ReadFile(".github/workflows/test.yml")
	if err != nil {
		t.Fatal(err)
	}

	text := string(workflow)
	for _, required := range []string{"actions/checkout@v7", "actions/setup-go@v7"} {
		if !strings.Contains(text, required) {
			t.Errorf("workflow is missing %s", required)
		}
	}
	for _, stale := range []string{"actions/checkout@v4", "actions/setup-go@v5"} {
		if strings.Contains(text, stale) {
			t.Errorf("workflow still contains deprecated %s", stale)
		}
	}
}
