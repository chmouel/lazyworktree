package app

import "github.com/chmouel/lazyworktree/internal/app/screen"

// appIconProviderBridge adapts the app's IconProvider to the screen package's interface.
type appIconProviderBridge struct{}

func (b *appIconProviderBridge) GetPRIcon() string {
	return getIconPR()
}

func (b *appIconProviderBridge) GetIssueIcon() string {
	return getIconIssue()
}

func (b *appIconProviderBridge) GetCIIcon(conclusion string) string {
	return ciIconForConclusion(conclusion)
}

// init sets up the icon provider bridge for the screen package.
func init() {
	screen.SetIconProvider(&appIconProviderBridge{})
}
