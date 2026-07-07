package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	appservices "github.com/chmouel/lazyworktree/internal/app/services"
	"github.com/chmouel/lazyworktree/internal/config"
	"github.com/chmouel/lazyworktree/internal/models"
)

type cleanupGitService interface {
	gitService
	GetMainBranch(ctx context.Context) string
	GetMergedBranches(ctx context.Context, baseBranch string) []string
}

type cleanupCandidateKind int

const (
	cleanupWorktree cleanupCandidateKind = iota
	cleanupBranch
	cleanupOrphan
)

type cleanupCandidate struct {
	kind       cleanupCandidateKind
	worktree   *models.WorktreeInfo
	branch     string
	source     string
	orphanPath string
}

type cleanupResult struct {
	worktrees int
	branches  int
	orphans   int
	failures  int
	items     []CleanupItem
}

// Cleanup item kinds as emitted in the structured summary.
const (
	CleanupKindWorktree = "worktree"
	CleanupKindBranch   = "branch"
	CleanupKindOrphan   = "orphan"
)

// CleanupItem describes a single candidate acted upon during cleanup.
type CleanupItem struct {
	Kind          string
	Path          string
	Branch        string
	Source        string
	BranchDeleted bool
	Failed        bool
	Error         string
}

// CleanupSummary reports the aggregate counts and per-item detail of a cleanup
// run so callers can render human-readable or machine-readable output.
type CleanupSummary struct {
	Worktrees int
	Branches  int
	Orphans   int
	Failures  int
	Items     []CleanupItem
}

// Cleanup finds merged worktrees, configured stale branches, and orphaned
// worktree directories. It prompts for a numbered selection unless all is true.
// When jsonOutput is true it requires all (non-interactive) and returns a
// structured summary for machine-readable rendering while suppressing progress
// messages such as terminate command notices.
func Cleanup(
	ctx context.Context,
	gitSvc cleanupGitService,
	cfg *config.AppConfig,
	all bool,
	jsonOutput bool,
	stdin io.Reader,
	stderr io.Writer,
) (CleanupSummary, error) {
	if jsonOutput && !all {
		return CleanupSummary{}, fmt.Errorf("--json requires --all")
	}

	candidates, repoDir, err := findCleanupCandidates(ctx, gitSvc, cfg, stderr)
	if err != nil {
		return CleanupSummary{}, err
	}
	if len(candidates) == 0 {
		if !jsonOutput {
			fmt.Fprintln(stderr, "Nothing to clean up.")
		}
		return CleanupSummary{}, nil
	}

	selected := candidates
	if !all {
		selected, err = promptCleanupSelection(candidates, stdin, stderr)
		if err != nil {
			return CleanupSummary{}, err
		}
		if len(selected) == 0 {
			fmt.Fprintln(stderr, "Cleanup cancelled.")
			return CleanupSummary{}, nil
		}
	}

	result := executeCleanup(ctx, gitSvc, cfg, repoDir, selected, jsonOutput, stderr)
	summary := CleanupSummary{
		Worktrees: result.worktrees,
		Branches:  result.branches,
		Orphans:   result.orphans,
		Failures:  result.failures,
		Items:     result.items,
	}
	if !jsonOutput {
		fmt.Fprintln(stderr, formatCleanupResult(result))
	}
	if result.failures > 0 {
		return summary, fmt.Errorf("cleanup completed with %d failure(s)", result.failures)
	}
	return summary, nil
}

func findCleanupCandidates(
	ctx context.Context,
	gitSvc cleanupGitService,
	cfg *config.AppConfig,
	stderr io.Writer,
) ([]cleanupCandidate, string, error) {
	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to get worktrees: %w", err)
	}

	if !cfg.DisablePR {
		for _, wt := range worktrees {
			if wt == nil || wt.IsMain {
				continue
			}
			pr, fetchErr := gitSvc.FetchPRForWorktreeWithError(ctx, wt.Path)
			if fetchErr != nil {
				fmt.Fprintf(stderr, "Warning: failed to inspect PR/MR for %s: %v\n", wt.Branch, fetchErr)
				continue
			}
			wt.PR = pr
		}
	}

	mainBranch := gitSvc.GetMainBranch(ctx)
	mergedBranches := gitSvc.GetMergedBranches(ctx, mainBranch)
	pruneCandidates := appservices.FindPruneCandidates(worktrees, mergedBranches, cfg.PruneStaleBranches)

	candidates := make([]cleanupCandidate, 0, len(pruneCandidates))
	for _, candidate := range pruneCandidates {
		kind := cleanupWorktree
		if candidate.Worktree == nil {
			kind = cleanupBranch
		}
		candidates = append(candidates, cleanupCandidate{
			kind:     kind,
			worktree: candidate.Worktree,
			branch:   candidate.Branch,
			source:   candidate.Source,
		})
	}

	repoDir := cleanupRepoWorktreeDir(ctx, gitSvc, cfg)
	for _, orphanPath := range findCleanupOrphans(repoDir, worktrees) {
		candidates = append(candidates, cleanupCandidate{
			kind:       cleanupOrphan,
			orphanPath: orphanPath,
		})
	}

	slices.SortFunc(candidates, func(a, b cleanupCandidate) int {
		return strings.Compare(a.sortKey(), b.sortKey())
	})
	return candidates, repoDir, nil
}

func cleanupRepoWorktreeDir(ctx context.Context, gitSvc cleanupGitService, cfg *config.AppConfig) string {
	if IsRepoLocal(cfg.WorktreeDir, gitSvc.GetMainWorktreePath(ctx)) {
		return cfg.WorktreeDir
	}
	return filepath.Join(cfg.WorktreeDir, gitSvc.ResolveRepoName(ctx))
}

func findCleanupOrphans(repoDir string, worktrees []*models.WorktreeInfo) []string {
	validPaths := make(map[string]struct{}, len(worktrees))
	for _, wt := range worktrees {
		if wt != nil {
			validPaths[normaliseCleanupPath(wt.Path)] = struct{}{}
		}
	}
	if len(validPaths) == 0 {
		return nil
	}

	entries, err := os.ReadDir(repoDir)
	if err != nil {
		return nil
	}

	orphans := make([]string, 0)
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		path := filepath.Join(repoDir, entry.Name())
		if _, ok := validPaths[normaliseCleanupPath(path)]; !ok {
			orphans = append(orphans, path)
		}
	}
	slices.Sort(orphans)
	return orphans
}

func normaliseCleanupPath(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return filepath.Clean(path)
	}
	return filepath.Clean(resolved)
}

func promptCleanupSelection(candidates []cleanupCandidate, stdin io.Reader, stderr io.Writer) ([]cleanupCandidate, error) {
	fmt.Fprintln(stderr, "Cleanup candidates:")
	fmt.Fprintln(stderr)
	for i, candidate := range candidates {
		fmt.Fprintf(stderr, "  [%d] %s\n", i+1, candidate.description())
	}
	fmt.Fprintln(stderr)
	fmt.Fprintf(stderr, "Select items (for example 1,3-5 or all; Enter cancels): ")

	scanner := bufio.NewScanner(stdin)
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read cleanup selection: %w", err)
		}
		return nil, nil
	}

	selection := strings.TrimSpace(scanner.Text())
	if selection == "" {
		return nil, nil
	}
	indices, err := parseCleanupSelection(selection, len(candidates))
	if err != nil {
		return nil, err
	}

	selected := make([]cleanupCandidate, 0, len(indices))
	for _, index := range indices {
		selected = append(selected, candidates[index-1])
	}
	return selected, nil
}

func parseCleanupSelection(selection string, itemCount int) ([]int, error) {
	selection = strings.TrimSpace(strings.ToLower(selection))
	if selection == "all" || selection == "*" {
		indices := make([]int, itemCount)
		for i := range itemCount {
			indices[i] = i + 1
		}
		return indices, nil
	}

	seen := make(map[int]struct{})
	for _, part := range strings.Split(selection, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			return nil, fmt.Errorf("invalid cleanup selection %q", selection)
		}

		start, end, isRange, err := parseCleanupRange(part)
		if err != nil {
			return nil, err
		}
		if !isRange {
			end = start
		}
		if start < 1 || end > itemCount {
			return nil, fmt.Errorf("selection %q is out of range (must be 1-%d)", part, itemCount)
		}
		for index := start; index <= end; index++ {
			seen[index] = struct{}{}
		}
	}

	indices := make([]int, 0, len(seen))
	for index := range seen {
		indices = append(indices, index)
	}
	slices.Sort(indices)
	return indices, nil
}

func parseCleanupRange(part string) (start, end int, isRange bool, err error) {
	if !strings.Contains(part, "-") {
		value, convErr := strconv.Atoi(part)
		if convErr != nil {
			return 0, 0, false, fmt.Errorf("invalid cleanup selection %q", part)
		}
		return value, value, false, nil
	}

	bounds := strings.Split(part, "-")
	if len(bounds) != 2 {
		return 0, 0, false, fmt.Errorf("invalid cleanup range %q", part)
	}
	start, err = strconv.Atoi(strings.TrimSpace(bounds[0]))
	if err != nil {
		return 0, 0, false, fmt.Errorf("invalid cleanup range %q", part)
	}
	end, err = strconv.Atoi(strings.TrimSpace(bounds[1]))
	if err != nil || start > end {
		return 0, 0, false, fmt.Errorf("invalid cleanup range %q", part)
	}
	return start, end, true, nil
}

func executeCleanup(
	ctx context.Context,
	gitSvc cleanupGitService,
	cfg *config.AppConfig,
	repoDir string,
	candidates []cleanupCandidate,
	silent bool,
	stderr io.Writer,
) cleanupResult {
	gitSvc.RunGit(ctx, []string{"git", "worktree", "prune"}, "", []int{0}, true, true)

	validPaths, validPathsOK := refreshedCleanupPaths(ctx, gitSvc)
	result := cleanupResult{}
	for _, candidate := range candidates {
		switch candidate.kind {
		case cleanupWorktree:
			runCleanupTerminateCommands(ctx, gitSvc, cfg, candidate.worktree, silent, stderr)
			removed := gitSvc.RunCommandChecked(
				ctx,
				[]string{"git", "worktree", "remove", "--force", candidate.worktree.Path},
				"",
				fmt.Sprintf("Failed to remove worktree %s", candidate.worktree.Path),
			)
			branchDeleted := gitSvc.RunCommandChecked(
				ctx,
				[]string{"git", "branch", "-D", candidate.branch},
				"",
				fmt.Sprintf("Failed to delete branch %s", candidate.branch),
			)
			item := CleanupItem{
				Kind:          CleanupKindWorktree,
				Path:          candidate.worktree.Path,
				Branch:        candidate.branch,
				Source:        candidate.source,
				BranchDeleted: branchDeleted,
			}
			if removed && branchDeleted {
				result.worktrees++
			} else {
				result.failures++
				item.Failed = true
				item.Error = worktreeCleanupError(removed, branchDeleted)
			}
			result.items = append(result.items, item)
		case cleanupBranch:
			deleted := gitSvc.RunCommandChecked(
				ctx,
				[]string{"git", "branch", "-D", candidate.branch},
				"",
				fmt.Sprintf("Failed to delete branch %s", candidate.branch),
			)
			item := CleanupItem{
				Kind:          CleanupKindBranch,
				Branch:        candidate.branch,
				Source:        candidate.source,
				BranchDeleted: deleted,
			}
			if deleted {
				result.branches++
			} else {
				result.failures++
				item.Failed = true
				item.Error = fmt.Sprintf("failed to delete branch %s", candidate.branch)
			}
			result.items = append(result.items, item)
		case cleanupOrphan:
			item := CleanupItem{
				Kind: CleanupKindOrphan,
				Path: candidate.orphanPath,
			}
			if !validPathsOK || !safeCleanupOrphan(candidate.orphanPath, repoDir, validPaths) {
				fmt.Fprintf(stderr, "Warning: skipped orphaned directory %s because it could not be revalidated safely\n", candidate.orphanPath)
				result.failures++
				item.Failed = true
				item.Error = "could not be revalidated safely"
				result.items = append(result.items, item)
				continue
			}
			if err := os.RemoveAll(candidate.orphanPath); err != nil {
				fmt.Fprintf(stderr, "Warning: failed to remove orphaned directory %s: %v\n", candidate.orphanPath, err)
				result.failures++
				item.Failed = true
				item.Error = err.Error()
			} else {
				result.orphans++
			}
			result.items = append(result.items, item)
		}
	}
	return result
}

// worktreeCleanupError describes which step of a worktree cleanup failed.
func worktreeCleanupError(removed, branchDeleted bool) string {
	switch {
	case !removed && !branchDeleted:
		return "failed to remove worktree and delete branch"
	case !removed:
		return "failed to remove worktree"
	default:
		return "failed to delete branch"
	}
}

func runCleanupTerminateCommands(
	ctx context.Context,
	gitSvc cleanupGitService,
	cfg *config.AppConfig,
	wt *models.WorktreeInfo,
	silent bool,
	stderr io.Writer,
) {
	var lazyCtxProvider func() appservices.LazyWorktreeContext
	if !cfg.DisablePR {
		lazyCtxProvider = func() appservices.LazyWorktreeContext {
			return lazyWorktreeContextForWorktree(ctx, gitSvc, wt)
		}
	}
	if err := runTerminateCommands(ctx, gitSvc, cfg, wt.Branch, wt.Path, lazyCtxProvider, silent); err != nil {
		fmt.Fprintf(stderr, "Warning: terminate commands failed for %s: %v\n", wt.Branch, err)
	}
}

func refreshedCleanupPaths(ctx context.Context, gitSvc cleanupGitService) (map[string]struct{}, bool) {
	worktrees, err := gitSvc.GetWorktrees(ctx)
	if err != nil || len(worktrees) == 0 {
		return nil, false
	}
	paths := make(map[string]struct{}, len(worktrees))
	for _, wt := range worktrees {
		if wt != nil {
			paths[normaliseCleanupPath(wt.Path)] = struct{}{}
		}
	}
	return paths, len(paths) > 0
}

func safeCleanupOrphan(path, repoDir string, validPaths map[string]struct{}) bool {
	normalisedPath := normaliseCleanupPath(path)
	if _, exists := validPaths[normalisedPath]; exists {
		return false
	}

	rel, err := filepath.Rel(normaliseCleanupPath(repoDir), normalisedPath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return false
	}
	return filepath.Dir(rel) == "."
}

func (c cleanupCandidate) sortKey() string {
	switch c.kind {
	case cleanupWorktree:
		return "0:" + strings.ToLower(c.branch)
	case cleanupBranch:
		return "1:" + strings.ToLower(c.branch)
	default:
		return "2:" + strings.ToLower(c.orphanPath)
	}
}

func (c cleanupCandidate) description() string {
	switch c.kind {
	case cleanupWorktree:
		status := sourceDescription(c.source)
		if c.worktree.Dirty || c.worktree.Untracked > 0 || c.worktree.Modified > 0 || c.worktree.Staged > 0 {
			status += "; HAS UNCOMMITTED CHANGES"
		}
		return fmt.Sprintf("worktree %s (branch %s; %s)", filepath.Base(c.worktree.Path), c.branch, status)
	case cleanupBranch:
		return fmt.Sprintf("branch %s (merged, no worktree)", c.branch)
	default:
		return fmt.Sprintf("orphaned directory %s", c.orphanPath)
	}
}

func sourceDescription(source string) string {
	switch source {
	case "pr":
		return "PR/MR merged"
	case "git":
		return "branch merged"
	default:
		return "PR/MR and branch merged"
	}
}

func formatCleanupResult(result cleanupResult) string {
	parts := make([]string, 0, 4)
	if result.worktrees > 0 {
		parts = append(parts, fmt.Sprintf("%d merged %s removed", result.worktrees, pluralise(result.worktrees, "worktree", "worktrees")))
	}
	if result.branches > 0 {
		parts = append(parts, fmt.Sprintf("%d stale %s deleted", result.branches, pluralise(result.branches, "branch", "branches")))
	}
	if result.orphans > 0 {
		parts = append(parts, fmt.Sprintf("%d orphaned %s removed", result.orphans, pluralise(result.orphans, "directory", "directories")))
	}
	if result.failures > 0 {
		parts = append(parts, fmt.Sprintf("%d %s", result.failures, pluralise(result.failures, "failure", "failures")))
	}
	if len(parts) == 0 {
		return "Nothing was cleaned up."
	}
	return "Cleanup complete: " + strings.Join(parts, ", ") + "."
}

func pluralise(count int, singular, plural string) string {
	if count == 1 {
		return singular
	}
	return plural
}
