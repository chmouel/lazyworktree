package models

// StatusFile represents a file entry from git status.
type StatusFile struct {
	Filename    string
	Status      string // XY status code (e.g., ".M", "M.", " ?")
	IsUntracked bool
}
