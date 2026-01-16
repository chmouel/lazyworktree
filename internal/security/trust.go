// Package security manages trust decisions and persistence for repository config files.
package security

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/chmouel/lazyworktree/internal/utils"
)

// TrustStatus represents the outcome of a trust check on a file.
type TrustStatus int

const (
	// TrustStatusTrusted indicates the file matches a known hash.
	TrustStatusTrusted TrustStatus = iota
	// TrustStatusUntrusted means the file either changed or has not been trusted yet.
	TrustStatusUntrusted
	// TrustStatusNotFound is returned when the file does not exist.
	TrustStatusNotFound
)

func getTrustDBPath() string {
	if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
		return filepath.Join(xdgDataHome, "lazyworktree", "trusted.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "lazyworktree", "trusted.json")
}

// TrustManager stores trusted hashes and enforces TOFU (Trust On First Use).
type TrustManager struct {
	mu            sync.RWMutex
	dbPath        string
	trustedHashes map[string]string // Map absolute path -> sha256 hash
}

// NewTrustManager creates and loads the persisted trust database.
func NewTrustManager() *TrustManager {
	tm := &TrustManager{
		dbPath:        getTrustDBPath(),
		trustedHashes: make(map[string]string),
	}
	tm.load()
	return tm
}

func (tm *TrustManager) load() {
	if _, err := os.Stat(tm.dbPath); os.IsNotExist(err) {
		return
	}

	data, err := os.ReadFile(tm.dbPath)
	if err != nil {
		return
	}

	tm.mu.Lock()
	defer tm.mu.Unlock()

	if err := json.Unmarshal(data, &tm.trustedHashes); err != nil {
		// If corrupt, start fresh for safety
		tm.trustedHashes = make(map[string]string)
	}
}

const defaultFilePerms = 0o600

func (tm *TrustManager) save() error {
	tm.mu.RLock()
	data, err := json.MarshalIndent(tm.trustedHashes, "", "  ")
	tm.mu.RUnlock()
	if err != nil {
		return err
	}

	dir := filepath.Dir(tm.dbPath)
	if err := os.MkdirAll(dir, utils.DefaultDirPerms); err != nil {
		return err
	}

	return os.WriteFile(tm.dbPath, data, defaultFilePerms)
}

// calculateHash calculates SHA256 of the file content
func (tm *TrustManager) calculateHash(filePath string) string {
	resolvedPath, err := filepath.Abs(filePath)
	if err != nil {
		return ""
	}

	// #nosec G304 -- resolvedPath is an absolute path derived from trusted input
	file, err := os.Open(resolvedPath)
	if err != nil {
		return ""
	}
	defer func() {
		_ = file.Close()
	}()

	hash := sha256.New()
	buf := make([]byte, 65536)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			hash.Write(buf[:n])
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return ""
		}
	}

	return fmt.Sprintf("%x", hash.Sum(nil))
}

// CheckTrust validates the given file path against the trust database using TOFU (Trust On First Use).
// Returns TrustStatusTrusted if the file hash matches a previously trusted hash,
// TrustStatusUntrusted if the file is new or has changed, or TrustStatusNotFound if the file doesn't exist.
func (tm *TrustManager) CheckTrust(filePath string) TrustStatus {
	resolvedPath, err := filepath.Abs(filePath)
	if err != nil {
		return TrustStatusNotFound
	}

	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return TrustStatusNotFound
	}

	currentHash := tm.calculateHash(resolvedPath)
	if currentHash == "" {
		return TrustStatusUntrusted
	}

	tm.mu.RLock()
	storedHash, exists := tm.trustedHashes[resolvedPath]
	tm.mu.RUnlock()

	if !exists {
		return TrustStatusUntrusted
	}

	if storedHash == currentHash {
		return TrustStatusTrusted
	}

	return TrustStatusUntrusted
}

// TrustFile records the current hash of a file as trusted and persists it to disk.
// Once trusted, the file's commands will run automatically until the file content changes.
func (tm *TrustManager) TrustFile(filePath string) error {
	resolvedPath, err := filepath.Abs(filePath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(resolvedPath); os.IsNotExist(err) {
		return fmt.Errorf("file does not exist: %s", resolvedPath)
	}

	currentHash := tm.calculateHash(resolvedPath)
	if currentHash == "" {
		return fmt.Errorf("failed to calculate hash for: %s", resolvedPath)
	}

	tm.mu.Lock()
	tm.trustedHashes[resolvedPath] = currentHash
	tm.mu.Unlock()

	return tm.save()
}
