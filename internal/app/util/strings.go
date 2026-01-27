package util

import "strings"

// CommitMeta is parsed commit metadata from a log entry.
type CommitMeta struct {
	SHA     string
	Author  string
	Email   string
	Date    string
	Subject string
	Body    []string
}

// AuthorInitials returns initials for an author name.
func AuthorInitials(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return ""
	}
	if len(fields) == 1 {
		runes := []rune(fields[0])
		if len(runes) <= 2 {
			return string(runes)
		}
		return string(runes[:2])
	}
	first := []rune(fields[0])
	last := []rune(fields[len(fields)-1])
	if len(first) == 0 || len(last) == 0 {
		return ""
	}
	return string([]rune{first[0], last[0]})
}

// ParseCommitMeta parses a raw commit metadata string into its fields.
func ParseCommitMeta(raw string) CommitMeta {
	parts := strings.Split(raw, "\x1f")
	meta := CommitMeta{}
	if len(parts) > 0 {
		meta.SHA = parts[0]
	}
	if len(parts) > 1 {
		meta.Author = parts[1]
	}
	if len(parts) > 2 {
		meta.Email = parts[2]
	}
	if len(parts) > 3 {
		meta.Date = parts[3]
	}
	if len(parts) > 4 {
		meta.Subject = parts[4]
	}
	if len(parts) > 5 {
		meta.Body = strings.Split(parts[5], "\n")
	}
	return meta
}
