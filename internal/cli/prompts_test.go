package cli

import (
	"testing"

	"github.com/chmouel/lazyworktree/internal/models"
)

func TestFormatWorktreeForList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		wt   *models.WorktreeInfo
		want string
	}{
		{
			name: "clean no commits",
			wt:   &models.WorktreeInfo{Branch: "main"},
			want: "main (clean)",
		},
		{
			name: "dirty ahead",
			wt:   &models.WorktreeInfo{Branch: "feature", Dirty: true, Ahead: 2},
			want: "feature (dirty, 2 commits ahead)",
		},
		{
			name: "clean behind",
			wt:   &models.WorktreeInfo{Branch: "bugfix", Behind: 1},
			want: "bugfix (clean, 1 commits behind)",
		},
		{
			name: "ahead and behind",
			wt:   &models.WorktreeInfo{Branch: "topic", Ahead: 3, Behind: 4},
			want: "topic (clean, 3 commits ahead, 4 behind)",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := formatWorktreeForList(tt.wt)
			if got != tt.want {
				t.Fatalf("unexpected formatting: want=%q got=%q", tt.want, got)
			}
		})
	}
}
