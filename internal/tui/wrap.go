package tui

import "strings"

func wordWrap(s string, width int) string {
	if width < 10 {
		width = 10
	}
	var out strings.Builder
	for i, para := range strings.Split(s, "\n") {
		if i > 0 {
			out.WriteString("\n")
		}
		if para == "" {
			continue
		}
		words := strings.Fields(para)
		lineLen := 0
		for wi, w := range words {
			for len(w) > width {
				if lineLen > 0 {
					out.WriteString("\n")
					lineLen = 0
				}
				out.WriteString(w[:width])
				w = w[width:]
				out.WriteString("\n")
			}
			if lineLen == 0 {
				out.WriteString(w)
				lineLen = len(w)
			} else if lineLen+1+len(w) <= width {
				out.WriteString(" " + w)
				lineLen += 1 + len(w)
			} else {
				out.WriteString("\n" + w)
				lineLen = len(w)
			}
			_ = wi
		}
	}
	return out.String()
}

func wrapMessage(prefix, body string, width int) string {
	const prefixWidth = 6
	bodyWidth := width - prefixWidth
	if bodyWidth < 10 {
		bodyWidth = 10
	}
	wrapped := wordWrap(body, bodyWidth)
	lines := strings.Split(wrapped, "\n")
	var b strings.Builder
	b.WriteString(prefix + lines[0])
	for _, l := range lines[1:] {
		b.WriteString("\n" + strings.Repeat(" ", prefixWidth) + l)
	}
	return b.String()
}
