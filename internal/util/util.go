package util

import (
	"fmt"
	"html/template"
	"strings"
)

func EnrichPost(body string) string {
	var b strings.Builder
	for _, rawLine := range strings.Split(body, "\n") {
		var outLine string

		for i, rawWord := range strings.Split(rawLine, " ") {
			var outWord string
			if strings.HasPrefix(rawWord, ">>") {
				postId := strings.TrimPrefix(rawWord, ">>")
				esc := template.HTMLEscapeString(rawWord)
				outWord = fmt.Sprintf(
					`<a onclick="onReplyLinkClick(event)" 
						 onmouseover="highlightPost(%[1]s, event)" `+
						`onmouseleave="highlightPost(%[1]s, event, false)" `+
						`href="#post-%[1]s" class="reply-link">%s</a>`,
					postId, esc,
				)
			} else {
				outWord = template.HTMLEscapeString(rawWord)
			}

			if i != 0 {
				outLine += " "
			}
			outLine += outWord
		}

		if strings.HasPrefix(rawLine, ">") && !strings.HasPrefix(rawLine, ">>") {
			outLine = `<span class="greentext">` + outLine + `</span>`
		}

		b.WriteString(outLine)
		b.WriteString("<br/>")
	}
	return b.String()
}
