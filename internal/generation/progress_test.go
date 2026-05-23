package generation

import "testing"

func TestStepFromJobResolvesOptionalAssetsStep(t *testing.T) {
	step := StepForJob(JobKindSite, "assets.fetch")
	if step == nil {
		t.Fatal("expected assets.fetch to resolve")
	}
	if step.Name != "assets.fetch" {
		t.Fatalf("expected assets.fetch, got %q", step.Name)
	}
}

func TestStepFromJobKeepsPageRepromptTotals(t *testing.T) {
	step := StepForJob(JobKindPageReprompt, "persist")
	if step == nil {
		t.Fatal("expected persist to resolve")
	}
	if step.Total != 5 {
		t.Fatalf("expected page reprompt to have 5 steps, got %d", step.Total)
	}
}

func TestStepFromJobKeepsThemeRegenerateScopedSteps(t *testing.T) {
	step := StepForJob(JobKindThemeRegenerate, "persist")
	if step == nil {
		t.Fatal("expected persist to resolve")
	}
	if step.Total != 4 {
		t.Fatalf("expected theme regenerate to have 4 steps, got %d", step.Total)
	}
	if StepForJob(JobKindThemeRegenerate, "copy.write") != nil {
		t.Fatal("did not expect copy.write to exist for theme regenerate")
	}
}
