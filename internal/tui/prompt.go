package tui

import (
	"fmt"

	"github.com/charmbracelet/huh"
	"github.com/sahilm/fuzzy"
)

// CreateInputs holds the inputs needed to create a new task workspace.
// Fields may be pre-filled from CLI flags; empty fields are collected interactively.
type CreateInputs struct {
	Name        string
	Description string
	JiraTicket  string
	Repos       []string
	ContextDocs []string
}

// NeedsPrompt returns the list of input fields that are missing and need prompting.
// Only required fields (name, description) are reported; optional fields (jira, repos,
// context-docs) are not included since they have sensible empty defaults.
func NeedsPrompt(prefilled CreateInputs) []string {
	var needed []string
	if prefilled.Name == "" {
		needed = append(needed, "name")
	}
	if prefilled.Description == "" {
		needed = append(needed, "description")
	}
	return needed
}

// FuzzyMatchRepos returns repoNames filtered and ranked by fuzzy match against query.
// An empty query returns all repos in their original order.
func FuzzyMatchRepos(repoNames []string, query string) []string {
	if query == "" {
		out := make([]string, len(repoNames))
		copy(out, repoNames)
		return out
	}
	matches := fuzzy.Find(query, repoNames)
	out := make([]string, len(matches))
	for i, m := range matches {
		out[i] = m.Str
	}
	return out
}

// PromptCreate runs interactive prompts to fill in missing fields.
// repoNames is the list of available repos (from ~/projects/repos/) for fuzzy selection.
// Fields already set in prefilled are skipped.
func PromptCreate(repoNames []string, prefilled CreateInputs) (CreateInputs, error) {
	result := prefilled

	var groups []*huh.Group

	// Text inputs for name, description, jira
	var nameField, descField, jiraField *huh.Input
	var inputFields []huh.Field

	if result.Name == "" {
		nameField = huh.NewInput().
			Title("Task name").
			Description("Short identifier (letters, numbers, _, -, .)").
			Value(&result.Name).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("name is required")
				}
				return nil
			})
		inputFields = append(inputFields, nameField)
	}

	if result.Description == "" {
		descField = huh.NewInput().
			Title("Description").
			Description("Brief description of the task").
			Value(&result.Description).
			Validate(func(s string) error {
				if s == "" {
					return fmt.Errorf("description is required")
				}
				return nil
			})
		inputFields = append(inputFields, descField)
	}

	if result.JiraTicket == "" {
		jiraField = huh.NewInput().
			Title("JIRA ticket (optional)").
			Description("e.g., RNA-549 (leave blank if none)").
			Value(&result.JiraTicket)
		inputFields = append(inputFields, jiraField)
	}

	if len(inputFields) > 0 {
		groups = append(groups, huh.NewGroup(inputFields...))
	}

	// Multi-select for repos, only if not already provided via flag
	if len(result.Repos) == 0 && len(repoNames) > 0 {
		options := make([]huh.Option[string], len(repoNames))
		for i, name := range repoNames {
			options[i] = huh.NewOption(name, name)
		}
		repoField := huh.NewMultiSelect[string]().
			Title("Select repositories").
			Description("Space to toggle, enter to confirm. Type to filter.").
			Options(options...).
			Filterable(true).
			Value(&result.Repos)
		groups = append(groups, huh.NewGroup(repoField))
	}

	if len(groups) == 0 {
		return result, nil
	}

	form := huh.NewForm(groups...)
	if err := form.Run(); err != nil {
		return result, fmt.Errorf("prompt: %w", err)
	}

	return result, nil
}
