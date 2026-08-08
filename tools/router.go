package tools

import (
	"context"
	"fmt"
)

// Option is one thing a Classify call can pick — a name plus enough
// description for the classifier to judge relevance.
type Option struct {
	Name        string
	Description string
}

// Classify picks the single best-matching option's Name for a
// plain-language request, given the request and the list of options —
// used for both the manifest and capability stage of Router.Resolve.
// Pluggable: Maya's default implementation is a single small OpenAI text
// call (see internal/realtime/router.go), but this signature doesn't
// hardcode OpenAI or any particular model size — a smaller or local model
// could implement it later with no change to Router itself.
type Classify func(ctx context.Context, request string, options []Option) (string, error)

// Router resolves a plain-language request to one Capability across all
// registered Manifests, in two classification calls — manifest, then
// capability within it — rather than exposing every capability as its own
// realtime tool. This is what keeps a realtime session's tool list small
// and fixed-size regardless of how many manifests or capabilities exist
// behind it.
type Router struct {
	manifests []Manifest
	classify  Classify
}

// NewRouter builds a Router over the given manifests, using classify for
// both stages of resolution.
func NewRouter(classify Classify, manifests ...Manifest) *Router {
	return &Router{manifests: manifests, classify: classify}
}

// Resolve classifies request down to one Capability and calls its Handler
// with request as the argument, returning its result. Returns a plain
// "nothing available" message (not an error) when no manifest or
// capability matches confidently — errors are reserved for the underlying
// Classify calls themselves failing, not for "nothing matched." With zero
// manifests registered, Resolve short-circuits immediately with no
// Classify calls at all.
func (r *Router) Resolve(ctx context.Context, request string) (string, error) {
	if len(r.manifests) == 0 {
		return "No connected actions are available yet.", nil
	}

	manifestOptions := make([]Option, len(r.manifests))
	for i, m := range r.manifests {
		manifestOptions[i] = Option{Name: m.Name, Description: m.Description}
	}
	manifestName, err := r.classify(ctx, request, manifestOptions)
	if err != nil {
		return "", fmt.Errorf("classify manifest: %w", err)
	}

	var manifest *Manifest
	for i := range r.manifests {
		if r.manifests[i].Name == manifestName {
			manifest = &r.manifests[i]
			break
		}
	}
	if manifest == nil {
		return "I don't have anything connected that can do that yet.", nil
	}
	if len(manifest.Capabilities) == 0 {
		return fmt.Sprintf("%s doesn't support anything yet.", manifest.Name), nil
	}

	capabilityOptions := make([]Option, len(manifest.Capabilities))
	for i, c := range manifest.Capabilities {
		capabilityOptions[i] = Option{Name: c.Name, Description: c.Description}
	}
	capabilityName, err := r.classify(ctx, request, capabilityOptions)
	if err != nil {
		return "", fmt.Errorf("classify capability: %w", err)
	}

	for _, c := range manifest.Capabilities {
		if c.Name == capabilityName {
			return c.Handler(ctx, request)
		}
	}
	return fmt.Sprintf("%s doesn't support that yet.", manifest.Name), nil
}
