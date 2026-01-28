package screen

// UIIcon represents an icon constant for UI elements.
type UIIcon int

// UI icon constants.
const (
	UIIconPRSelect UIIcon = iota
	UIIconIssueSelect
	UIIconListSelect
)

// iconProvider interface for getting icons.
type iconProvider interface {
	GetPRIcon() string
	GetIssueIcon() string
	GetCIIcon(conclusion string) string
}

// defaultIconProvider provides fallback ASCII icons.
type defaultIconProvider struct{}

func (p *defaultIconProvider) GetPRIcon() string {
	return "PR"
}

func (p *defaultIconProvider) GetIssueIcon() string {
	return "ISS"
}

func (p *defaultIconProvider) GetCIIcon(conclusion string) string {
	return ""
}

var currentIconProvider iconProvider = &defaultIconProvider{}

// SetIconProvider allows the app package to inject the real icon provider.
func SetIconProvider(provider iconProvider) {
	currentIconProvider = provider
}

// getIconPR returns the PR icon from the current provider.
func getIconPR() string {
	return currentIconProvider.GetPRIcon()
}

// getIconIssue returns the issue icon from the current provider.
func getIconIssue() string {
	return currentIconProvider.GetIssueIcon()
}

// iconWithSpace adds a space after an icon if not empty.
func iconWithSpace(icon string) string {
	if icon == "" {
		return ""
	}
	return icon + " "
}

// labelWithIcon returns a label prefixed with an icon (if icons enabled).
func labelWithIcon(icon UIIcon, label string, showIcons bool) string {
	if !showIcons {
		return label
	}
	var iconStr string
	switch icon {
	case UIIconPRSelect:
		iconStr = getIconPR()
	case UIIconIssueSelect:
		iconStr = getIconIssue()
	}
	return iconWithSpace(iconStr) + label
}

// getCIStatusIcon returns a CI status indicator icon.
func getCIStatusIcon(ciStatus string, isDraft, showIcons bool) string {
	if isDraft {
		return "D"
	}
	if showIcons {
		if icon := currentIconProvider.GetCIIcon(ciStatus); icon != "" {
			return icon
		}
	}
	switch ciStatus {
	case "success":
		return "S"
	case "failure":
		return "F"
	case "skipped":
		return "-"
	case "cancelled":
		return "C"
	case "pending":
		return "P"
	default:
		return "?"
	}
}
