package commands

import "github.com/chmouel/lazyworktree/internal/app/services"

const defaultMRUSectionLabel = "Recently Used"

// PaletteItem represents a palette entry.
type PaletteItem struct {
	ID          string
	Label       string
	Description string
	IsSection   bool
	IsMRU       bool
}

// PaletteOptions controls palette item building.
type PaletteOptions struct {
	MRUEnabled      bool
	MRULimit        int
	History         []services.CommandPaletteUsage
	Actions         []CommandAction
	CustomItems     []PaletteItem
	MRUSectionLabel string
}

// BuildPaletteItems builds palette items from actions and history.
func BuildPaletteItems(opts PaletteOptions) []PaletteItem {
	items := make([]PaletteItem, 0, len(opts.Actions)+len(opts.CustomItems)+10)
	itemMap := make(map[string]PaletteItem)

	for _, action := range opts.Actions {
		if action.ID == "" {
			continue
		}
		itemMap[action.ID] = PaletteItem{
			ID:          action.ID,
			Label:       action.Label,
			Description: action.Description,
		}
	}

	for _, item := range opts.CustomItems {
		if item.ID != "" && !item.IsSection {
			itemMap[item.ID] = item
		}
	}

	mruItems := buildMRUItems(opts, itemMap)
	mruIDs := make(map[string]bool)
	if len(mruItems) > 0 {
		label := opts.MRUSectionLabel
		if label == "" {
			label = defaultMRUSectionLabel
		}
		items = append(items, PaletteItem{Label: label, IsSection: true})
		items = append(items, mruItems...)
		for _, item := range mruItems {
			if item.ID != "" {
				mruIDs[item.ID] = true
			}
		}
	}

	currentSection := ""
	for _, action := range opts.Actions {
		if action.Section != "" && action.Section != currentSection {
			items = append(items, PaletteItem{Label: action.Section, IsSection: true})
			currentSection = action.Section
		}
		if action.ID == "" || mruIDs[action.ID] {
			continue
		}
		if action.Available != nil && !action.Available() {
			continue
		}
		items = append(items, PaletteItem{
			ID:          action.ID,
			Label:       action.Label,
			Description: action.Description,
		})
	}

	for _, item := range opts.CustomItems {
		if item.ID == "" || !mruIDs[item.ID] {
			items = append(items, item)
		}
	}

	return items
}

func buildMRUItems(opts PaletteOptions, itemMap map[string]PaletteItem) []PaletteItem {
	if !opts.MRUEnabled || len(opts.History) == 0 {
		return nil
	}

	limit := opts.MRULimit
	if limit <= 0 {
		return nil
	}

	mruItems := make([]PaletteItem, 0, limit)
	for _, usage := range opts.History {
		if len(mruItems) >= limit {
			break
		}
		item, exists := itemMap[usage.ID]
		if !exists {
			continue
		}
		item.IsMRU = true
		mruItems = append(mruItems, item)
	}
	return mruItems
}
