package crawler

import (
	"crawler/internal/domain"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// ExtractPageData parses HTML content and extracts relevant data.
func ExtractPageData(url, htmlContent string) (*domain.PageData, error) {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	data := &domain.PageData{
		URL:      url,
		Title:    doc.Find("title").First().Text(),
		MetaTags: make(map[string]string),
		Images:   []string{},
		Headers:  []string{},
		Status:   "completed",
	}

	// Extract Meta Tags
	doc.Find("meta").Each(func(i int, s *goquery.Selection) {
		name, _ := s.Attr("name")
		property, _ := s.Attr("property")
		content, _ := s.Attr("content")
		key := name
		if property != "" {
			key = property
		}
		if key != "" && content != "" {
			data.MetaTags[key] = content
		}
	})

	// Extract Headers
	doc.Find("h1, h2, h3").Each(func(i int, s *goquery.Selection) {
		data.Headers = append(data.Headers, s.Text())
	})

	// Extract Images
	doc.Find("img").Each(func(i int, s *goquery.Selection) {
		src, exists := s.Attr("src")
		if exists && src != "" {
			data.Images = append(data.Images, src)
		}
	})

	// Extract clean body text content
	doc.Find("script, style").Each(func(i int, s *goquery.Selection) {
		s.Remove()
	})
	data.Content = strings.TrimSpace(doc.Find("body").Text())

	return data, nil
}
