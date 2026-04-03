package cmd

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
)

func TestRenderMarkdownTextFormatsCodeAndInlineCode(t *testing.T) {
	text := "Use `orch run` first.\n```go\nfmt.Println(\"hi\")\n```"
	rendered := renderMarkdownText(text)
	if !strings.Contains(rendered, "orch run") {
		t.Fatalf("expected inline code text in render: %q", rendered)
	}
	if !strings.Contains(rendered, "fmt.Println(\"hi\")") {
		t.Fatalf("expected fenced code text in render: %q", rendered)
	}
}

func TestRefreshViewportContentRendersStructuredEntries(t *testing.T) {
	m := &interactiveModel{viewport: viewport.New(120, 40)}
	m.entries = []chatEntry{
		{Kind: chatEntryUser, Title: "You", Lines: []string{"selam"}},
		{Kind: chatEntryAssistant, Title: "Orch", Lines: []string{"# Title", "- item", "```txt", "code", "```"}},
	}
	m.refreshViewportContent()
	view := m.viewport.View()
	for _, want := range []string{"selam", "Title", "item", "code"} {
		if !strings.Contains(view, want) {
			t.Fatalf("expected %q in viewport render: %q", want, view)
		}
	}
}
