package agent

import (
	"strings"
	"testing"

	"github.com/yetone/magpie/internal/catalog"
)

func TestDshFullWindowOutputLeavesCompactionBudget(t *testing.T) {
	for _, tc := range []struct {
		name  string
		model catalog.Model
		want  int
	}{
		{"full-window catalog", catalog.Model{ID: "space-bunny-free", Context: 524288, Output: 524288}, 65536},
		{"output above window", catalog.Model{ID: "above", Context: 524288, Output: 1048576}, 65536},
		{"ordinary catalog", catalog.Model{ID: "ordinary", Context: 1000000, Output: 128000}, 128000},
		{"user output", catalog.Model{ID: "user", Context: 524288, Output: 98304, OutputSet: true}, 98304},
		{"explicit full window", catalog.Model{ID: "user-full", Context: 524288, Output: 524288, OutputSet: true}, 524288},
		{"unknown window", catalog.Model{ID: "unknown", Output: 128000}, 128000},
		{"unknown output", catalog.Model{ID: "no-output", Context: 524288}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, kind := range dshKinds {
				got := dshKindConfig(kind, []catalog.Model{tc.model}, "", "http://127.0.0.1:3425").Models[0]
				if got.MaxTokens != tc.want || got.ContextWindow != tc.model.Context {
					t.Fatalf("%s: context=%d output=%d; want context=%d output=%d", kind.key, got.ContextWindow, got.MaxTokens, tc.model.Context, tc.want)
				}
			}
			lines := strings.Join(dshProviderLines("high", []catalog.Model{tc.model}, "http://127.0.0.1:3425"), "\n")
			if tc.want == 65536 && !strings.Contains(lines, "maxTokens: 65536") {
				t.Fatalf("legacy route still reserves its full window: %s", lines)
			}
		})
	}
}
