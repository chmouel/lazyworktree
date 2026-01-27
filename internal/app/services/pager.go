package services

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

// Pager defines how diffs and content are displayed.
type Pager interface {
	Show(content string, env []string, cwd string) tea.Cmd
	ShowFileDiff(sf models.StatusFile, wt *models.WorktreeInfo) tea.Cmd
	ShowCommitDiff(sha string, wt *models.WorktreeInfo) tea.Cmd
}

// PagerCommand determines the pager command to use.
func PagerCommand(cfg *config.AppConfig) string {
	if cfg != nil {
		if pager := strings.TrimSpace(cfg.Pager); pager != "" {
			return pager
		}
	}
	if pager := strings.TrimSpace(os.Getenv("PAGER")); pager != "" {
		return pager
	}
	if _, err := exec.LookPath("less"); err == nil {
		return "less --use-color -q --wordwrap -qcR -P 'Press q to exit..'"
	}
	if _, err := exec.LookPath("more"); err == nil {
		return "more"
	}
	return "cat"
}

// EditorCommand determines the editor command to use.
func EditorCommand(cfg *config.AppConfig) string {
	if cfg != nil {
		if editor := strings.TrimSpace(cfg.Editor); editor != "" {
			return os.ExpandEnv(editor)
		}
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return editor
	}
	if _, err := exec.LookPath("nvim"); err == nil {
		return "nvim"
	}
	if _, err := exec.LookPath("vi"); err == nil {
		return "vi"
	}
	return ""
}

// PagerEnv returns environment variables needed for the pager.
func PagerEnv(pager string) string {
	if pagerIsLess(pager) {
		return "LESS= LESSHISTFILE=-"
	}
	return ""
}

func pagerIsLess(pager string) bool {
	fields := strings.FieldsSeq(pager)
	for field := range fields {
		if strings.Contains(field, "=") && !strings.HasPrefix(field, "-") && !strings.Contains(field, "/") {
			continue
		}
		return filepath.Base(field) == "less"
	}
	return false
}
