package multiplexer

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/chmouel/lazyworktree/internal/app/services"
)

// Target is an existing multiplexer session or workspace. Key is the
// LazyWorktree worktree tag, empty when the target is untagged.
type Target struct {
	ID   string
	Name string
	Key  string
}

// Resolution describes the existing target selected for a worktree.
type Resolution struct {
	Target Target
	Found  bool
	// Ambiguous reports several equally good matches: targets carrying the
	// worktree tag when none uses the requested name, or untagged targets
	// sharing a name. Target is only a stable pick among them.
	Ambiguous bool
	// Legacy reports a match by name on an untagged target.
	Legacy bool
}

var (
	// ErrTargetConflict reports a name already used by another worktree.
	ErrTargetConflict = errors.New("multiplexer name conflict")
	// ErrAmbiguousTarget reports several targets tagged for one worktree.
	ErrAmbiguousTarget = errors.New("ambiguous multiplexer target")
)

// WorktreeKey returns the identity recorded for a worktree path. Symlinks are
// not resolved and the path does not need to exist.
func WorktreeKey(path string) string {
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return filepath.Clean(path)
}

// HerdrWorktreeKey returns a worktree identity short enough for Herdr workspace
// tokens, which truncate values to 80 characters.
func HerdrWorktreeKey(path string) string {
	sum := sha256.Sum256([]byte(WorktreeKey(path)))
	return "v1:" + hex.EncodeToString(sum[:])[:32]
}

// maxDefaultSessionName caps default names built from pull request titles.
const maxDefaultSessionName = 40

// DefaultSessionName is the session name used when session_name is not
// configured: "#<number> <title>" for pull request worktrees, with the title
// shortened to fit maxDefaultSessionName characters, otherwise the worktree
// name. Tags, not names, find sessions again, so no prefix is needed.
func DefaultSessionName(env map[string]string, worktreePath string) string {
	number := strings.TrimSpace(env[services.EnvLazyWorktreeNumber])
	title := strings.Join(strings.Fields(env[services.EnvLazyWorktreeTitle]), " ")
	if number != "" && title != "" {
		prefix := "#" + number + " "
		return prefix + shortenTitle(title, maxDefaultSessionName-len([]rune(prefix)))
	}
	if name := strings.TrimSpace(env[services.EnvWorktreeName]); name != "" {
		return name
	}
	return filepath.Base(worktreePath)
}

// shortenTitle fits title into limit characters, cutting at the last word
// boundary when there is one and marking the cut with an ellipsis.
func shortenTitle(title string, limit int) string {
	runes := []rune(title)
	if len(runes) <= limit {
		return title
	}
	if limit < 2 {
		limit = 2
	}
	cut := string(runes[:limit-1])
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:-") + "…"
}

// LegacyDefaultNames lists the names earlier releases gave a worktree's session
// when session_name was not configured, so those untagged sessions are reused.
func LegacyDefaultNames(prefix, worktreePath string) []string {
	base := filepath.Base(worktreePath)
	if prefix == "wt:" {
		return []string{prefix + base}
	}
	return []string{prefix + base, "wt:" + base}
}

func targetConflictError(name string) error {
	return fmt.Errorf("%w: %q is used by another worktree; change session_name or use on_exists: new", ErrTargetConflict, name)
}

func ambiguousTargetError(names []string) error {
	return fmt.Errorf("%w: several sessions belong to this worktree (%v); close the extra ones or use on_exists: new", ErrAmbiguousTarget, names)
}

// ResolveTarget selects the existing target for a worktree. Tagged targets win
// over names; a name only matches an untagged target or one with the same tag.
// An empty key falls back to matching by name alone. legacyNames are tried last
// and only match untagged targets.
func ResolveTarget(targets []Target, key, name string, legacyNames ...string) (Resolution, error) {
	if key != "" {
		var tagged []Target
		for _, target := range targets {
			if target.Key == key {
				tagged = append(tagged, target)
			}
		}
		switch len(tagged) {
		case 0:
		case 1:
			return Resolution{Target: tagged[0], Found: true}, nil
		default:
			var named []Target
			for _, target := range tagged {
				if target.Name == name {
					named = append(named, target)
				}
			}
			if len(named) > 0 {
				return pickByName(named, false), nil
			}
			sort.SliceStable(tagged, func(i, j int) bool { return tagged[i].Name < tagged[j].Name })
			return Resolution{Target: tagged[0], Found: true, Ambiguous: true}, nil
		}
	}

	var named []Target
	for _, target := range targets {
		if target.Name != name {
			continue
		}
		// A worktree without a tag (see TmuxWorktreeKey) owns no tagged target.
		if target.Key != "" {
			return Resolution{}, targetConflictError(name)
		}
		named = append(named, target)
	}
	if len(named) > 0 {
		return pickByName(named, key != ""), nil
	}
	if key != "" {
		var matches []Target
		for _, target := range targets {
			if target.Key == "" && target.Name != "" && slices.Contains(legacyNames, target.Name) {
				matches = append(matches, target)
			}
		}
		if len(matches) > 0 {
			return pickByName(matches, true), nil
		}
	}
	return Resolution{}, nil
}

// pickByName selects among targets matching by name. Herdr allows
// duplicate labels and legacy names have several forms, so several matches are
// ambiguous and the lowest ID is picked so the choice is stable.
func pickByName(matches []Target, legacy bool) Resolution {
	sort.SliceStable(matches, func(i, j int) bool { return matches[i].ID < matches[j].ID })
	return Resolution{Target: matches[0], Found: true, Legacy: legacy, Ambiguous: len(matches) > 1}
}

// TaggedNames lists the names of every target carrying key, for error messages.
func TaggedNames(targets []Target, key string) []string {
	var names []string
	for _, target := range targets {
		if target.Key == key {
			names = append(names, target.Name)
		}
	}
	sort.Strings(names)
	return names
}

// CheckReplacementName reports a conflict when name is held by any target
// other than the one about to be replaced, so nothing is destroyed for a
// replacement that could not take the name.
func CheckReplacementName(targets []Target, name string, replacing Target) error {
	for _, target := range targets {
		if target.Name == name && target.ID != replacing.ID {
			return targetConflictError(name)
		}
	}
	return nil
}

// ResolveForKill resolves the target that on_exists: kill should replace,
// refusing ambiguous matches and names held by another target.
func ResolveForKill(targets []Target, key, name string, legacyNames ...string) (Resolution, error) {
	resolution, err := ResolveTarget(targets, key, name, legacyNames...)
	if err != nil || !resolution.Found {
		return resolution, err
	}
	if resolution.Ambiguous {
		if resolution.Target.Key == "" {
			return Resolution{}, fmt.Errorf("%w: several sessions are named %q; close the extra ones or use on_exists: new", ErrAmbiguousTarget, resolution.Target.Name)
		}
		return Resolution{}, ambiguousTargetError(TaggedNames(targets, key))
	}
	// Only legacy names prove an untagged session came from LazyWorktree; one
	// that merely shares the requested name may be the user's own.
	if resolution.Legacy && resolution.Target.Name == name {
		return Resolution{}, fmt.Errorf("%w: %q is an untagged session LazyWorktree will not close; close it yourself or use on_exists: switch", ErrTargetConflict, name)
	}
	if err := CheckReplacementName(targets, name, resolution.Target); err != nil {
		return Resolution{}, err
	}
	return resolution, nil
}
