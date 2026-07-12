package services

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// AgentWatchService watches agent transcript directories for JSONL changes.
type AgentWatchService struct {
	Started bool
	Waiting bool
	Roots   []string
	// SpoolRoots is the subset of Roots that contain hook spool .json files.
	// .json events are only accepted from these roots; all roots accept .jsonl.
	SpoolRoots []string
	Events     chan struct{}
	Done       chan struct{}
	Paths      map[string]struct{}
	Mu         sync.Mutex
	Watcher    *fsnotify.Watcher
	// Debounce throttles how often transcript-write bursts trigger a refresh.
	// A value <= 0 disables throttling. See PlanRefresh.
	Debounce    time.Duration
	LastRefresh time.Time
	// trailingScheduled is set while a single trailing-edge refresh is pending so
	// a burst schedules at most one catch-up. These timing fields are only
	// touched from the bubbletea update loop, never the watcher goroutine.
	trailingScheduled bool
	logf              func(string, ...any)
}

// RefreshPlan describes how a watcher event should be handled.
type RefreshPlan struct {
	// Now requests an immediate (leading-edge) refresh.
	Now bool
	// TrailingIn, when > 0, requests a single trailing-edge refresh scheduled
	// this far in the future so the final write of a burst still renders without
	// waiting for the periodic auto-refresh.
	TrailingIn time.Duration
}

// NewAgentWatchService creates a watcher for the provided roots. debounce
// bounds how often transcript-write events trigger a full session re-parse.
func NewAgentWatchService(roots []string, debounce time.Duration, logf func(string, ...any)) *AgentWatchService {
	return &AgentWatchService{
		Roots:    roots,
		Debounce: debounce,
		logf:     logf,
	}
}

// Start initialises the watcher and begins listening for transcript changes.
func (w *AgentWatchService) Start() (bool, error) {
	if w.Started {
		return false, nil
	}
	roots := make([]string, 0, len(w.Roots))
	for _, root := range w.Roots {
		if strings.TrimSpace(root) != "" {
			roots = append(roots, root)
		}
	}
	if len(roots) == 0 {
		return false, nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return false, err
	}

	w.Started = true
	w.Watcher = watcher
	w.Roots = roots
	w.Events = make(chan struct{}, 1)
	w.Done = make(chan struct{})
	w.Paths = make(map[string]struct{})
	for _, root := range w.Roots {
		w.addWatchTree(root)
	}
	go w.run()
	return true, nil
}

// Stop stops the watcher and closes resources.
func (w *AgentWatchService) Stop() {
	if !w.Started {
		return
	}
	close(w.Done)
	w.Started = false
	if w.Watcher != nil {
		_ = w.Watcher.Close()
	}
}

// NextEvent returns the next watcher event channel if a wait is not already active.
func (w *AgentWatchService) NextEvent() <-chan struct{} {
	if w.Events == nil || w.Waiting {
		return nil
	}
	w.Waiting = true
	return w.Events
}

// ResetWaiting clears the pending wait flag after an event is processed.
func (w *AgentWatchService) ResetWaiting() {
	w.Waiting = false
}

// PlanRefresh decides how a watcher event should be handled. An active agent
// appends to its transcript many times per second; without throttling every
// append triggers a full re-parse of every session JSONL and pegs a CPU core.
// Outside the debounce window the event refreshes immediately (leading edge).
// Inside it the immediate refresh is dropped, but the first such event arms a
// single trailing-edge refresh at the end of the window so the final write of a
// burst still renders promptly rather than waiting for the periodic
// auto-refresh. A Debounce <= 0 disables throttling.
func (w *AgentWatchService) PlanRefresh(now time.Time) RefreshPlan {
	if w.Debounce <= 0 {
		w.LastRefresh = now
		return RefreshPlan{Now: true}
	}
	if w.LastRefresh.IsZero() || now.Sub(w.LastRefresh) >= w.Debounce {
		w.LastRefresh = now
		return RefreshPlan{Now: true}
	}
	if w.trailingScheduled {
		return RefreshPlan{}
	}
	w.trailingScheduled = true
	return RefreshPlan{TrailingIn: w.Debounce - now.Sub(w.LastRefresh)}
}

// TrailingRefreshFired records that the scheduled trailing-edge refresh has run,
// clearing the pending flag and starting a fresh debounce window from now.
func (w *AgentWatchService) TrailingRefreshFired(now time.Time) {
	w.trailingScheduled = false
	w.LastRefresh = now
}

func (w *AgentWatchService) run() {
	for {
		select {
		case <-w.Done:
			return
		case event, ok := <-w.Watcher.Events:
			if !ok {
				return
			}
			if event.Op&fsnotify.Create != 0 {
				w.maybeWatchNewDir(event.Name)
			}
			if strings.HasSuffix(event.Name, ".jsonl") {
				w.signal()
			} else if strings.HasSuffix(event.Name, ".json") && w.isUnderSpoolRoot(event.Name) {
				w.signal()
			}
		case err, ok := <-w.Watcher.Errors:
			if !ok {
				return
			}
			if w.logf != nil {
				w.logf("agent watcher error: %v", err)
			}
		}
	}
}

func (w *AgentWatchService) addWatchTree(root string) {
	if root == "" {
		return
	}
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		w.addWatchDir(path)
		return nil
	})
}

func (w *AgentWatchService) maybeWatchNewDir(path string) {
	if !w.isUnderRoot(path) {
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	w.addWatchDir(path)
}

func (w *AgentWatchService) addWatchDir(path string) {
	if path == "" {
		return
	}
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return
	}
	w.Mu.Lock()
	defer w.Mu.Unlock()
	if _, ok := w.Paths[path]; ok {
		return
	}
	if err := w.Watcher.Add(path); err != nil {
		return
	}
	w.Paths[path] = struct{}{}
}

func (w *AgentWatchService) signal() {
	select {
	case <-w.Done:
		return
	default:
	}
	select {
	case w.Events <- struct{}{}:
	default:
	}
}

func (w *AgentWatchService) isUnderRoot(path string) bool {
	for _, root := range w.Roots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func (w *AgentWatchService) isUnderSpoolRoot(path string) bool {
	for _, root := range w.SpoolRoots {
		if path == root || strings.HasPrefix(path, root+string(filepath.Separator)) {
			return true
		}
	}
	return false
}
