package tools

import (
	"context"
	"errors"
	"testing"
)

func testManifests(called *[]string) []Manifest {
	return []Manifest{
		{
			Name:        "gmail",
			Description: "email",
			Capabilities: []Capability{
				{Name: "summarize_emails", Description: "summarize", Handler: func(_ context.Context, arguments string) (string, error) {
					*called = append(*called, "gmail.summarize_emails")
					return "summarized: " + arguments, nil
				}},
			},
		},
		{
			Name:        "upi",
			Description: "payments",
			Capabilities: []Capability{
				{Name: "pay_contact", Description: "pay", Handler: func(_ context.Context, arguments string) (string, error) {
					*called = append(*called, "upi.pay_contact")
					return "paid: " + arguments, nil
				}},
			},
		},
	}
}

func TestResolveTwoStageDispatch(t *testing.T) {
	var called []string
	request := "pay my roommate 500"
	router := NewRouter(twoStageFake(t, "upi", "pay_contact"), testManifests(&called)...)

	result, err := router.Resolve(context.Background(), request)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "paid: "+request {
		t.Fatalf("result = %q, want %q", result, "paid: "+request)
	}
	if len(called) != 1 || called[0] != "upi.pay_contact" {
		t.Fatalf("called = %v, want exactly [upi.pay_contact]", called)
	}
}

// twoStageFake returns a Classify that answers with manifestName the first
// time it's called and capabilityName every time after — modeling the
// real two-call sequence (manifest stage, then capability stage) without
// depending on request text to disambiguate which stage is active.
func twoStageFake(t *testing.T, manifestName, capabilityName string) Classify {
	t.Helper()
	calls := 0
	return func(_ context.Context, _ string, options []Option) (string, error) {
		calls++
		want := manifestName
		if calls > 1 {
			want = capabilityName
		}
		for _, o := range options {
			if o.Name == want {
				return o.Name, nil
			}
		}
		return want, nil
	}
}

func TestResolveNoMatchingManifest(t *testing.T) {
	var called []string
	router := NewRouter(twoStageFake(t, "nonexistent", "irrelevant"), testManifests(&called)...)

	result, err := router.Resolve(context.Background(), "do something unrelated")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "I don't have anything connected that can do that yet." {
		t.Fatalf("unexpected result: %q", result)
	}
	if len(called) != 0 {
		t.Fatalf("expected no handler called, got %v", called)
	}
}

func TestResolveNoMatchingCapability(t *testing.T) {
	var called []string
	router := NewRouter(twoStageFake(t, "gmail", "nonexistent_capability"), testManifests(&called)...)

	result, err := router.Resolve(context.Background(), "do something gmail-ish but unsupported")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "gmail doesn't support that yet." {
		t.Fatalf("unexpected result: %q", result)
	}
	if len(called) != 0 {
		t.Fatalf("expected no handler called, got %v", called)
	}
}

func TestResolveZeroManifestsMakesNoClassifyCalls(t *testing.T) {
	classifyCalls := 0
	classify := func(_ context.Context, _ string, _ []Option) (string, error) {
		classifyCalls++
		return "", errors.New("should not be called")
	}
	router := NewRouter(classify)

	result, err := router.Resolve(context.Background(), "anything")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if result != "No connected actions are available yet." {
		t.Fatalf("unexpected result: %q", result)
	}
	if classifyCalls != 0 {
		t.Fatalf("classify called %d times, want 0", classifyCalls)
	}
}

func TestResolveClassifyErrorPropagates(t *testing.T) {
	var called []string
	classify := func(_ context.Context, _ string, _ []Option) (string, error) {
		return "", errors.New("boom")
	}
	router := NewRouter(classify, testManifests(&called)...)

	_, err := router.Resolve(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}
