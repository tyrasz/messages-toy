package services

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

// DocumentMetadata contains extracted document information
type DocumentMetadata struct {
	PageCount int    // Number of pages (for PDF)
	Title     string // Document title if available
}

// DocumentService handles document processing operations
type DocumentService struct {
	pdfInfoPath string // Path to pdfinfo (from poppler-utils)
}

// NewDocumentService creates a new DocumentService
func NewDocumentService() *DocumentService {
	// Look for pdfinfo in PATH (part of poppler-utils)
	pdfInfoPath, _ := exec.LookPath("pdfinfo")

	return &DocumentService{
		pdfInfoPath: pdfInfoPath,
	}
}

// IsAvailable checks if pdfinfo is installed
func (s *DocumentService) IsAvailable() bool {
	return s.pdfInfoPath != ""
}

// ExtractPDFMetadata extracts metadata from a PDF file
func (s *DocumentService) ExtractPDFMetadata(pdfPath string) (*DocumentMetadata, error) {
	// Try using pdfinfo first
	if s.pdfInfoPath != "" {
		return s.extractWithPdfInfo(pdfPath)
	}

	// Fallback to manual PDF parsing
	return s.extractManually(pdfPath)
}

// extractWithPdfInfo uses the pdfinfo command to get metadata
func (s *DocumentService) extractWithPdfInfo(pdfPath string) (*DocumentMetadata, error) {
	cmd := exec.Command(s.pdfInfoPath, pdfPath)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("pdfinfo failed: %w", err)
	}

	metadata := &DocumentMetadata{}
	scanner := bufio.NewScanner(bytes.NewReader(output))

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "Pages:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				if count, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil {
					metadata.PageCount = count
				}
			}
		} else if strings.HasPrefix(line, "Title:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				metadata.Title = strings.TrimSpace(parts[1])
			}
		}
	}

	return metadata, nil
}

// extractManually parses the PDF file to find page count
// This is a fallback when pdfinfo is not available
func (s *DocumentService) extractManually(pdfPath string) (*DocumentMetadata, error) {
	file, err := os.Open(pdfPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Read up to 1MB to find page count
	buf := make([]byte, 1024*1024)
	n, err := file.Read(buf)
	if err != nil {
		return nil, err
	}
	content := string(buf[:n])

	metadata := &DocumentMetadata{}

	// Look for /Count N in the PDF which typically indicates page count
	// This pattern matches the Pages object /Count field
	re := regexp.MustCompile(`/Type\s*/Pages[^>]*?/Count\s+(\d+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		if count, err := strconv.Atoi(matches[1]); err == nil {
			metadata.PageCount = count
		}
	}

	// Alternative pattern - count Page objects directly
	if metadata.PageCount == 0 {
		pageCount := strings.Count(content, "/Type /Page")
		// Don't count /Type /Pages
		pagesCount := strings.Count(content, "/Type /Pages")
		metadata.PageCount = pageCount - pagesCount
	}

	return metadata, nil
}

// GetDocumentType returns a readable document type based on content type
func GetDocumentType(contentType string) string {
	ct := strings.ToLower(contentType)
	switch {
	case strings.Contains(ct, "pdf"):
		return "PDF"
	case strings.Contains(ct, "word"):
		return "Word Document"
	case strings.Contains(ct, "excel"), strings.Contains(ct, "spreadsheet"):
		return "Spreadsheet"
	case strings.Contains(ct, "text/plain"):
		return "Text File"
	default:
		return "Document"
	}
}
