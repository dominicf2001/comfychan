package util

import (
	"html/template"
	"strings"
)

func EnrichPost(body string) string {
	lines := strings.Split(body, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, ">") {
			escaped := template.HTMLEscapeString(line)
			lines[i] = `<span class="greentext">` + escaped + `</span>`
		} else {
			lines[i] = template.HTMLEscapeString(line)
		}
	}
	return strings.Join(lines, "<br />")
}
