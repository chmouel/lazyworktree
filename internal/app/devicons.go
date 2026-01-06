package app

import (
	"os"
	"time"

	devicons "github.com/epilande/go-devicons"
)

type iconFileInfo struct {
	name  string
	isDir bool
}

func (i iconFileInfo) Name() string { return i.name }

func (i iconFileInfo) Size() int64 { return 0 }

func (i iconFileInfo) Mode() os.FileMode {
	if i.isDir {
		return os.ModeDir | 0o755
	}
	return 0
}

func (i iconFileInfo) ModTime() time.Time { return time.Time{} }

func (i iconFileInfo) IsDir() bool { return i.isDir }

func (i iconFileInfo) Sys() any { return nil }

const (
	iconPR = ""

	iconCISuccess   = ""
	iconCIFailure   = ""
	iconCIPending   = ""
	iconCISkipped   = ""
	iconCICancelled = ""
	iconCIUnknown   = ""
)

func deviconForName(name string, isDir bool) string {
	if name == "" {
		return ""
	}
	style := devicons.IconForInfo(iconFileInfo{name: name, isDir: isDir})
	return style.Icon
}

func ciIconForConclusion(conclusion string) string {
	switch conclusion {
	case "success":
		return iconCISuccess
	case "failure":
		return iconCIFailure
	case "skipped":
		return iconCISkipped
	case "cancelled":
		return iconCICancelled
	case "pending", "":
		return iconCIPending
	default:
		return iconCIUnknown
	}
}

func iconWithSpace(icon string) string {
	if icon == "" {
		return ""
	}
	return icon + " "
}
