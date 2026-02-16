package app

import "strings"

const (
	osc8OpenPrefix = "\x1b]8;;"
	osc8Close      = "\x1b\\"
)

// osc8Hyperlink wraps text in an OSC-8 hyperlink sequence when URL is non-empty.
func osc8Hyperlink(text, url string) string {
	url = strings.TrimSpace(url)
	if text == "" || url == "" {
		return text
	}

	return osc8OpenPrefix + url + osc8Close + text + osc8OpenPrefix + osc8Close
}
