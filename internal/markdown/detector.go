package markdown

import (
	"regexp"
	"strings"
)

// IsMarkdownContentType checks if the Content-Type header indicates markdown.
func IsMarkdownContentType(contentType string) bool {
	ct := strings.ToLower(contentType)
	return strings.HasPrefix(ct, "text/markdown") ||
		strings.HasPrefix(ct, "text/x-markdown")
}

// IsMarkdownURL checks if the URL indicates a markdown file.
func IsMarkdownURL(url string) bool {
	lower := strings.ToLower(url)
	return strings.HasSuffix(lower, ".md") ||
		strings.HasSuffix(lower, ".markdown")
}

// IsMarkdownContent uses heuristics to detect if content is markdown.
func IsMarkdownContent(content string) bool {
	if content == "" {
		return false
	}

	trimmed := strings.TrimSpace(content)

	// If it looks like HTML, it's not markdown
	if looksLikeHTML(trimmed) {
		return false
	}

	// Check for common markdown patterns
	return hasMarkdownPatterns(trimmed)
}

// looksLikeHTML checks if content appears to be HTML.
func looksLikeHTML(content string) bool {
	lower := strings.ToLower(content)
	return strings.HasPrefix(lower, "<!doctype") ||
		strings.HasPrefix(lower, "<html") ||
		strings.HasPrefix(lower, "<head") ||
		strings.HasPrefix(lower, "<body")
}

// hasMarkdownPatterns checks for common markdown syntax.
func hasMarkdownPatterns(content string) bool {
	// Check for headers (# Title)
	if regexp.MustCompile(`^#{1,6}\s+\S`).MatchString(content) {
		return true
	}

	// Check for unordered lists (- item or * item)
	if regexp.MustCompile(`(?m)^[\-\*]\s+\S`).MatchString(content) {
		return true
	}

	// Check for markdown links [text](url)
	if regexp.MustCompile(`\[.+?\]\(.+?\)`).MatchString(content) {
		return true
	}

	return false
}

// MarkdownURLVariants returns potential markdown versions of a URL.
// Returns empty slice if URL is already a markdown file (except GitHub blob URLs).
func MarkdownURLVariants(url string) []string {
	var variants []string

	// GitHub blob → raw conversion (even if already .md, we want the raw URL)
	if strings.Contains(url, "github.com") && strings.Contains(url, "/blob/") {
		raw := strings.Replace(url, "github.com", "raw.githubusercontent.com", 1)
		raw = strings.Replace(raw, "/blob/", "/", 1)
		variants = append(variants, raw)
		return variants
	}

	// Already markdown? No variants needed
	if IsMarkdownURL(url) {
		return []string{}
	}

	// Default: try adding .md extension
	cleanURL := strings.TrimSuffix(url, "/")
	variants = append(variants, cleanURL+".md")

	return variants
}

// Detect combines all detection methods to determine if content is markdown.
// Checks in order: Content-Type, URL, then content heuristics.
func Detect(url, contentType, content string) bool {
	if IsMarkdownContentType(contentType) {
		return true
	}
	if IsMarkdownURL(url) {
		return true
	}
	return IsMarkdownContent(content)
}
