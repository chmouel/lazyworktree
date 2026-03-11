package services

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
)

// AgentWatchService watches agent transcript directories for JSONL changes.
type AgentWatchService struct {
	Started bool
	Waiting bool
	Roots   []string
	Events  chan struct{}
	Done    chan struct{}
	Paths   map[string]struct{}
	Mu      sync.Mutex
	Watcher *fsnotify.Watcher
	logf    func(string, ...any)
}

// NewAgentWatchService creates a watcher for the provided roots.
func NewAgentWatchService(roots []string, logf func(string, ...any)) *AgentWatchService {
	return &AgentWatchService{
		Roots: roots,
		logf:  logf,
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
			if !strings.HasSuffix(event.Name, ".jsonl") {
				continue
			}
			w.signal()
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
