package app

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestAggregateCIConclusion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		checks   []*models.CICheck
		expected string
	}{
		{
			name:     "all success",
			checks:   []*models.CICheck{{Conclusion: "success"}, {Conclusion: "success"}},
			expected: "success",
		},
		{
			name:     "failure takes priority",
			checks:   []*models.CICheck{{Conclusion: "success"}, {Conclusion: "failure"}, {Conclusion: "pending"}},
			expected: "failure",
		},
		{
			name:     "pending over success",
			checks:   []*models.CICheck{{Conclusion: "success"}, {Conclusion: "pending"}},
			expected: "pending",
		},
		{
			name:     "empty conclusion treated as pending",
			checks:   []*models.CICheck{{Conclusion: "success"}, {Conclusion: ""}},
			expected: "pending",
		},
		{
			name:     "all skipped",
			checks:   []*models.CICheck{{Conclusion: "skipped"}, {Conclusion: "cancelled"}},
			expected: "skipped",
		},
		{
			name:     "single failure",
			checks:   []*models.CICheck{{Conclusion: "failure"}},
			expected: "failure",
		},
		{
			name:     "single success",
			checks:   []*models.CICheck{{Conclusion: "success"}},
			expected: "success",
		},
		{
			name:     "skipped and success",
			checks:   []*models.CICheck{{Conclusion: "skipped"}, {Conclusion: "success"}},
			expected: "success",
		},
		{
			name:     "cancelled and pending",
			checks:   []*models.CICheck{{Conclusion: "cancelled"}, {Conclusion: ""}},
			expected: "pending",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := aggregateCIConclusion(tt.checks)
			assert.Equal(t, tt.expected, result)
		})
	}
}
