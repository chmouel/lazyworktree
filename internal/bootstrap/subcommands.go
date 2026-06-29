package bootstrap

import (
	"fmt"
	"os"
	"strings"

	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/git"
	"github.com/chmouel/lazyworktree/internal/log"
	"github.com/chmouel/lazyworktree/internal/utils"
)

// loadCLIConfig loads and configures the application configuration for CLI mode.
func loadCLIConfig(configFileFlag, worktreeDirFlag, debugLogFlag string, configOverrides []string) (*config.AppConfig, error) {
	ensureRepoPath()

	cfg, err := config.LoadConfig(configFileFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		cfg = config.DefaultConfig()
	}

	if err := applyWorktreeDirConfig(cfg, worktreeDirFlag); err != nil {
		return nil, err
	}

	if len(configOverrides) > 0 {
		if err := cfg.ApplyCLIOverrides(configOverrides); err != nil {
			return nil, fmt.Errorf("error applying config overrides: %w", err)
		}
	}

	configureCLIDebugLog(debugLogFlag, cfg)

	return cfg, nil
}

func configureCLIDebugLog(debugLogFlag string, cfg *config.AppConfig) {
	path := strings.TrimSpace(debugLogFlag)
	if path == "" && cfg != nil {
		path = strings.TrimSpace(cfg.DebugLog)
	}
	if path == "" {
		_ = log.SetFile("")
		return
	}

	expanded, err := utils.ExpandPath(path)
	if err == nil {
		path = expanded
	}
	if err := log.SetFile(path); err != nil {
		fmt.Fprintf(os.Stderr, "Error opening debug log file %q: %v\n", path, err)
		return
	}
	if cfg != nil {
		cfg.DebugLog = path
	}
	log.Printf("debug logging enabled")
}

// newCLIGitService creates a new git service configured for CLI mode.
func newCLIGitService(cfg *config.AppConfig) *git.Service {
	gitSvc := git.NewService(cliNotify, cliNotifyOnce)
	gitSvc.SetGitPager(cfg.GitPager)
	gitSvc.SetGitPagerArgs(cfg.GitPagerArgs)
	return gitSvc
}

// cliNotify is a notification callback for git operations in CLI mode.
func cliNotify(message, severity string) {
	if severity == "error" {
		fmt.Fprintf(os.Stderr, "Error: %s\n", message)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", message)
}

// cliNotifyOnce is a notification callback for git operations that should only fire once.
func cliNotifyOnce(_, message, severity string) {
	cliNotify(message, severity)
}
