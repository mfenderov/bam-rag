package processor

import (
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"
)

// Processor converts HTML content to Markdown.
type Processor struct{}

// New creates a new HTML to Markdown processor.
func New() *Processor {
	return &Processor{}
}

// Convert transforms HTML content into Markdown.
func (p *Processor) Convert(htmlContent string) (string, error) {
	if htmlContent == "" {
		return "", nil
	}

	markdown, err := htmltomarkdown.ConvertString(htmlContent)
	if err != nil {
		return "", err
	}

	// Clean up excessive whitespace
	markdown = strings.TrimSpace(markdown)
	return markdown, nil
}

// ExtractTitle extracts the <title> content from HTML.
func (p *Processor) ExtractTitle(htmlContent string) string {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return ""
	}

	var title string
	var findTitle func(*html.Node)
	findTitle = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "title" {
			if n.FirstChild != nil {
				title = n.FirstChild.Data
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findTitle(c)
		}
	}
	findTitle(doc)

	return strings.TrimSpace(title)
}
