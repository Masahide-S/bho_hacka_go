package ui

import (
	"strings"
)

// styleContent applies color based on content
func styleContent(content string) string {
	lines := strings.Split(content, "\n")
	var styledLines []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.Contains(line, "✓") {
			styledLines = append(styledLines, SuccessStyle.Render(line))
		} else if strings.Contains(line, "✗") {
			styledLines = append(styledLines, ErrorStyle.Render(line))
		} else if strings.HasPrefix(trimmed, "└─") ||
			strings.HasPrefix(trimmed, "-") ||
			strings.HasPrefix(trimmed, ":") {
			styledLines = append(styledLines, InfoStyle.Render(line))
		} else if strings.Contains(line, "|") {
			styledLines = append(styledLines, WarningStyle.Render(line))
		} else {
			styledLines = append(styledLines, line)
		}
	}

	return strings.Join(styledLines, "\n")
}
