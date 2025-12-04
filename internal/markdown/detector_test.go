package markdown

import (
	"testing"
)

func TestIsMarkdownContentType(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		want        bool
	}{
		{"text/markdown", "text/markdown", true},
		{"text/x-markdown", "text/x-markdown", true},
		{"text/markdown with charset", "text/markdown; charset=utf-8", true},
		{"text/html", "text/html", false},
		{"text/plain", "text/plain", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMarkdownContentType(tt.contentType); got != tt.want {
				t.Errorf("IsMarkdownContentType(%q) = %v, want %v", tt.contentType, got, tt.want)
			}
		})
	}
}

func TestIsMarkdownURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want bool
	}{
		{"ends with .md", "https://example.com/README.md", true},
		{"ends with .markdown", "https://example.com/doc.markdown", true},
		{"GitHub raw", "https://raw.githubusercontent.com/user/repo/main/file.md", true},
		{"regular HTML", "https://example.com/page.html", false},
		{"no extension", "https://example.com/docs/intro", false},
		{"md in path but not extension", "https://example.com/md/page", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMarkdownURL(tt.url); got != tt.want {
				t.Errorf("IsMarkdownURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestIsMarkdownContent(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{
			name:    "starts with h1",
			content: "# Hello World\n\nSome content here.",
			want:    true,
		},
		{
			name:    "starts with h2",
			content: "## Section\n\nContent.",
			want:    true,
		},
		{
			name:    "markdown with code block",
			content: "# Title\n\n```go\nfunc main() {}\n```",
			want:    true,
		},
		{
			name:    "HTML document",
			content: "<!DOCTYPE html><html><body>Content</body></html>",
			want:    false,
		},
		{
			name:    "HTML with head",
			content: "<html><head><title>Page</title></head><body>Hi</body></html>",
			want:    false,
		},
		{
			name:    "plain text no headers",
			content: "Just some plain text without any markdown.",
			want:    false,
		},
		{
			name:    "empty content",
			content: "",
			want:    false,
		},
		{
			name:    "markdown list",
			content: "- Item 1\n- Item 2\n- Item 3",
			want:    true,
		},
		{
			name:    "markdown links",
			content: "[Link text](https://example.com)\n\nMore content.",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsMarkdownContent(tt.content); got != tt.want {
				t.Errorf("IsMarkdownContent() = %v, want %v\nContent: %q", got, tt.want, tt.content)
			}
		})
	}
}

func TestMarkdownURLVariants(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want []string
	}{
		{
			name: "regular URL gets .md suffix",
			url:  "https://example.com/docs/intro",
			want: []string{
				"https://example.com/docs/intro.md",
			},
		},
		{
			name: "URL with trailing slash",
			url:  "https://example.com/docs/intro/",
			want: []string{
				"https://example.com/docs/intro.md",
			},
		},
		{
			name: "GitHub blob to raw",
			url:  "https://github.com/user/repo/blob/main/README.md",
			want: []string{
				"https://raw.githubusercontent.com/user/repo/main/README.md",
			},
		},
		{
			name: "GitHub non-blob URL",
			url:  "https://github.com/user/repo",
			want: []string{
				"https://github.com/user/repo.md",
			},
		},
		{
			name: "already .md URL returns empty",
			url:  "https://example.com/README.md",
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MarkdownURLVariants(tt.url)
			if len(got) != len(tt.want) {
				t.Errorf("MarkdownURLVariants(%q) = %v, want %v", tt.url, got, tt.want)
				return
			}
			for i, v := range got {
				if v != tt.want[i] {
					t.Errorf("MarkdownURLVariants(%q)[%d] = %q, want %q", tt.url, i, v, tt.want[i])
				}
			}
		})
	}
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		contentType string
		content     string
		want        bool
	}{
		{
			name:        "markdown by content-type",
			url:         "https://example.com/page",
			contentType: "text/markdown",
			content:     "random content",
			want:        true,
		},
		{
			name:        "markdown by url",
			url:         "https://example.com/README.md",
			contentType: "text/plain",
			content:     "random content",
			want:        true,
		},
		{
			name:        "markdown by content",
			url:         "https://example.com/page",
			contentType: "text/plain",
			content:     "# Title\n\nSome markdown content.",
			want:        true,
		},
		{
			name:        "html page",
			url:         "https://example.com/page.html",
			contentType: "text/html",
			content:     "<html><body>Content</body></html>",
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Detect(tt.url, tt.contentType, tt.content); got != tt.want {
				t.Errorf("Detect() = %v, want %v", got, tt.want)
			}
		})
	}
}
