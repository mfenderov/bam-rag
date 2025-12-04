package processor

import (
	"strings"
	"testing"
)

func TestProcessor_ConvertHTMLToMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		contains []string // Expected substrings in output
	}{
		{
			name: "converts headings",
			html: `<html><body><h1>Title</h1><h2>Subtitle</h2></body></html>`,
			contains: []string{
				"# Title",
				"## Subtitle",
			},
		},
		{
			name: "converts paragraphs",
			html: `<html><body><p>Hello world.</p><p>Second paragraph.</p></body></html>`,
			contains: []string{
				"Hello world.",
				"Second paragraph.",
			},
		},
		{
			name: "converts links",
			html: `<html><body><p>Check <a href="https://example.com">this link</a>.</p></body></html>`,
			contains: []string{
				"[this link](https://example.com)",
			},
		},
		{
			name: "converts code blocks",
			html: `<html><body><pre><code>func main() {}</code></pre></body></html>`,
			contains: []string{
				"func main() {}",
			},
		},
		{
			name: "converts inline code",
			html: `<html><body><p>Use <code>go run</code> to execute.</p></body></html>`,
			contains: []string{
				"`go run`",
			},
		},
		{
			name: "converts lists",
			html: `<html><body><ul><li>Item 1</li><li>Item 2</li></ul></body></html>`,
			contains: []string{
				"Item 1",
				"Item 2",
			},
		},
	}

	p := New()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := p.Convert(tt.html)
			if err != nil {
				t.Fatalf("Convert() error = %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(result, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, result)
				}
			}
		})
	}
}

func TestProcessor_ConvertHTMLToMarkdown_ExtractsTitle(t *testing.T) {
	html := `<html><head><title>Page Title</title></head><body><p>Content</p></body></html>`

	p := New()
	_, err := p.Convert(html)
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	title := p.ExtractTitle(html)
	if title != "Page Title" {
		t.Errorf("ExtractTitle() = %q, want %q", title, "Page Title")
	}
}

func TestProcessor_ConvertHTMLToMarkdown_EmptyInput(t *testing.T) {
	p := New()

	result, err := p.Convert("")
	if err != nil {
		t.Fatalf("Convert() error = %v", err)
	}

	// Should handle empty input gracefully
	if result == "" {
		// Empty output is acceptable for empty input
	}
}

func TestProcessor_ExtractTitle_NoTitle(t *testing.T) {
	p := New()
	html := `<html><body><p>No title here</p></body></html>`

	title := p.ExtractTitle(html)
	if title != "" {
		t.Errorf("ExtractTitle() should return empty for no title, got %q", title)
	}
}
