package slack

import (
	"slices"
	"strings"
	"unicode/utf8"
)

const (
	maxSectionTextLength  = 3000
	maxFieldTextLength    = 2000
	maxContextTextLength  = 3000
	maxImageAltTextLength = 2000
	truncationMarker      = "…"
)

func truncateText(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	head := s[:runeOffset(s, limit-utf8.RuneCountInString(truncationMarker))]
	if open := unclosedSegmentStart(head, s[len(head):]); open != -1 {
		head = head[:open]
	}
	return head + truncationMarker
}

func runeOffset(s string, n int) int {
	for i := range s {
		if n == 0 {
			return i
		}
		n--
	}
	return len(s)
}

func unclosedSegmentStart(head, tail string) int {
	open := strings.LastIndexByte(head, '<')
	if open == -1 || strings.IndexByte(head[open:], '>') != -1 {
		return -1
	}
	closeIdx := strings.IndexByte(tail, '>')
	if closeIdx == -1 || strings.IndexByte(tail[:closeIdx], '<') != -1 {
		return -1
	}
	return open
}

func truncateFields(fields []TextObject) []TextObject {
	var out []TextObject
	for i, f := range fields {
		text := truncateText(f.Text, maxFieldTextLength)
		if text == f.Text {
			continue
		}
		if out == nil {
			out = slices.Clone(fields)
		}
		out[i].Text = text
	}
	if out == nil {
		return fields
	}
	return out
}
