package tui_test

import (
	"testing"

	"github.com/brian-lai/clean-slate/internal/tui"
)

func TestNeedsPromptAllFilled(t *testing.T) {
	prefilled := tui.CreateInputs{
		Name:        "my-task",
		Description: "A description",
		JiraTicket:  "PROJ-549",
		Repos:       []string{"repo-a"},
		ContextDocs: []string{"/tmp/notes.txt"},
	}
	needed := tui.NeedsPrompt(prefilled)
	if len(needed) != 0 {
		t.Errorf("NeedsPrompt(all-filled) = %v, want empty", needed)
	}
}

func TestNeedsPromptMissingRequired(t *testing.T) {
	// Missing name and description (both required)
	prefilled := tui.CreateInputs{}
	needed := tui.NeedsPrompt(prefilled)

	// Expect at minimum: name, description
	has := map[string]bool{}
	for _, f := range needed {
		has[f] = true
	}
	if !has["name"] {
		t.Errorf("NeedsPrompt should request 'name' when empty, got %v", needed)
	}
	if !has["description"] {
		t.Errorf("NeedsPrompt should request 'description' when empty, got %v", needed)
	}
}

func TestNeedsPromptOptionalEmpty(t *testing.T) {
	// Name and description filled; jira/repos/context-docs empty (all optional)
	prefilled := tui.CreateInputs{
		Name:        "my-task",
		Description: "desc",
	}
	needed := tui.NeedsPrompt(prefilled)

	// Required fields are satisfied. Optional fields may still prompt — that's UX choice.
	// The contract is: required fields never prompt when filled.
	has := map[string]bool{}
	for _, f := range needed {
		has[f] = true
	}
	if has["name"] {
		t.Errorf("name is filled; should not be in needed list, got %v", needed)
	}
	if has["description"] {
		t.Errorf("description is filled; should not be in needed list, got %v", needed)
	}
}

func TestFuzzyMatchRepos(t *testing.T) {
	repos := []string{"alpha", "alpha-core", "alpha-core2", "bravo", "charlie", "delta"}

	// "al" should match alpha* repos first
	matches := tui.FuzzyMatchRepos(repos, "al")
	if len(matches) < 3 {
		t.Fatalf("FuzzyMatchRepos(al) returned %d matches, want >= 3 (alpha, alpha-core, alpha-core2)", len(matches))
	}

	// Top 3 results should all contain "al"
	for i := 0; i < 3; i++ {
		if matches[i] != "alpha" && matches[i] != "alpha-core" && matches[i] != "alpha-core2" {
			t.Errorf("FuzzyMatchRepos(al)[%d] = %q, want an alpha* repo in top 3", i, matches[i])
		}
	}
}

func TestFuzzyMatchReposNoMatch(t *testing.T) {
	repos := []string{"alpha", "bravo"}
	matches := tui.FuzzyMatchRepos(repos, "zzzzz")
	if len(matches) != 0 {
		t.Errorf("FuzzyMatchRepos(zzzzz) = %v, want empty", matches)
	}
}

func TestFuzzyMatchReposEmptyQuery(t *testing.T) {
	repos := []string{"alpha", "bravo", "delta"}
	matches := tui.FuzzyMatchRepos(repos, "")
	// Empty query returns all repos
	if len(matches) != len(repos) {
		t.Errorf("FuzzyMatchRepos(empty) len = %d, want %d", len(matches), len(repos))
	}
}
