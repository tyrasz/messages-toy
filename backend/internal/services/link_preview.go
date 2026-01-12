package services

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type LinkMetadata struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	FaviconURL  string `json:"favicon_url,omitempty"`
}

// FetchLinkMetadata fetches and parses metadata from a URL
func FetchLinkMetadata(targetURL string) (*LinkMetadata, error) {
	// Validate URL
	parsedURL, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, err
	}

	// Set a user agent to avoid being blocked
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; MessengerBot/1.0)")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Limit response size to 1MB
	limitedReader := io.LimitReader(resp.Body, 1024*1024)
	body, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}

	metadata := &LinkMetadata{
		URL: targetURL,
	}

	// Parse HTML
	doc, err := html.Parse(strings.NewReader(string(body)))
	if err != nil {
		return metadata, nil // Return partial metadata
	}

	// Extract metadata from HTML
	extractMetadata(doc, metadata)

	// Set default favicon URL if not found
	if metadata.FaviconURL == "" {
		metadata.FaviconURL = parsedURL.Scheme + "://" + parsedURL.Host + "/favicon.ico"
	}

	// Convert relative image URLs to absolute
	if metadata.ImageURL != "" && !strings.HasPrefix(metadata.ImageURL, "http") {
		if strings.HasPrefix(metadata.ImageURL, "//") {
			metadata.ImageURL = parsedURL.Scheme + ":" + metadata.ImageURL
		} else if strings.HasPrefix(metadata.ImageURL, "/") {
			metadata.ImageURL = parsedURL.Scheme + "://" + parsedURL.Host + metadata.ImageURL
		}
	}

	return metadata, nil
}

func extractMetadata(n *html.Node, metadata *LinkMetadata) {
	if n.Type == html.ElementNode {
		switch n.Data {
		case "title":
			if n.FirstChild != nil && metadata.Title == "" {
				metadata.Title = n.FirstChild.Data
			}
		case "meta":
			var name, property, content string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "name":
					name = attr.Val
				case "property":
					property = attr.Val
				case "content":
					content = attr.Val
				}
			}

			// OpenGraph tags
			switch property {
			case "og:title":
				if content != "" {
					metadata.Title = content
				}
			case "og:description":
				if content != "" {
					metadata.Description = content
				}
			case "og:image":
				if content != "" {
					metadata.ImageURL = content
				}
			case "og:site_name":
				if content != "" {
					metadata.SiteName = content
				}
			}

			// Twitter card tags (fallback)
			switch name {
			case "twitter:title":
				if metadata.Title == "" && content != "" {
					metadata.Title = content
				}
			case "twitter:description":
				if metadata.Description == "" && content != "" {
					metadata.Description = content
				}
			case "twitter:image":
				if metadata.ImageURL == "" && content != "" {
					metadata.ImageURL = content
				}
			case "description":
				if metadata.Description == "" && content != "" {
					metadata.Description = content
				}
			}

		case "link":
			var rel, href string
			for _, attr := range n.Attr {
				switch attr.Key {
				case "rel":
					rel = attr.Val
				case "href":
					href = attr.Val
				}
			}
			if (rel == "icon" || rel == "shortcut icon") && href != "" {
				metadata.FaviconURL = href
			}
		}
	}

	// Recurse into children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		extractMetadata(c, metadata)
	}
}
