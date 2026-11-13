package influencer

import (
	"strings"
	"testing"
)

// The turnaround layout is fixed: every panel the prompt promises must appear,
// and the reference-matching instruction must survive prompt assembly.
func TestBuildMultiViewPromptKeepsEveryPanel(t *testing.T) {
	prompt := BuildMultiViewPrompt(Selection{"gender": {"female"}, "hair": {"hair_bob"}}, "")

	for _, want := range []string{
		"Front view",
		"Three-quarter front view",
		"Side profile view",
		"Three-quarter back view",
		"Back view",
		"Front headshot, eyes gently closed",
		"Three-quarter headshot, eyes half-open",
		"Front headshot, eyes open looking straight into the lens",
		"16:9",
		"must match reference image exactly in every panel",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

func TestBuildMultiViewPromptCarriesCharacterAndBrief(t *testing.T) {
	prompt := BuildMultiViewPrompt(Selection{"gender": {"female"}}, "matching the client brief")

	for _, want := range []string{
		Fragment("gender", "female"),
		"matching the client brief",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}

// A brief-only parent has no picks, and an empty parent has neither. Both must
// still produce a usable prompt rather than an empty character clause.
func TestBuildMultiViewPromptFallsBackWithoutCharacterDetail(t *testing.T) {
	if got := BuildMultiViewPrompt(Selection{}, "  brief only  "); !strings.Contains(got, "brief only") {
		t.Fatalf("brief dropped:\n%s", got)
	}
	if got := BuildMultiViewPrompt(Selection{}, "   "); !strings.Contains(got, "the exact character") {
		t.Fatalf("missing fallback character clause:\n%s", got)
	}
}

// The tier lead from BuildPrompt would re-exaggerate an already-rendered
// character, so the turnaround prompt must not carry it.
func TestBuildMultiViewPromptOmitsTierLead(t *testing.T) {
	prompt := BuildMultiViewPrompt(Selection{"gender": {"female"}}, "")
	if strings.Contains(prompt, tierLead("total")) {
		t.Fatalf("turnaround prompt carried the tier lead:\n%s", prompt)
	}
}
