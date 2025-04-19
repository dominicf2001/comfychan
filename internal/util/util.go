package util

import (
	"fmt"
	"html/template"
	"strings"
)

func EnrichPost(body string) string {
	var b strings.Builder
	for _, raw := range strings.Split(body, "\n") {
		var out string
		switch {
		case strings.HasPrefix(raw, ">>"):
			postId := strings.TrimPrefix(raw, ">>")
			esc := template.HTMLEscapeString(raw)
			out = fmt.Sprintf(
				`<a  onclick="onReplyLinkClick(event)" 
					 onmouseover="highlightPost(%[1]s, event)" `+
					`onmouseleave="highlightPost(%[1]s, event, false)" `+
					`href="#post-%[1]s" class="reply-link">%s</a>`,
				postId, esc,
			)
		case strings.HasPrefix(raw, ">"):
			esc := template.HTMLEscapeString(raw)
			out = `<span class="greentext">` + esc + `</span>`
		default:
			out = template.HTMLEscapeString(raw)
		}

		b.WriteString(out)
		b.WriteString("<br />")
	}
	return b.String()
}
