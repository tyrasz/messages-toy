package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"

	vision "cloud.google.com/go/vision/apiv1"
	"cloud.google.com/go/vision/v2/apiv1/visionpb"
	"messenger/internal/models"
)

type ModerationService struct {
	client *vision.ImageAnnotatorClient
	// Set to true to use mock moderation (for development without GCP)
	useMock bool
}

type ScanResult struct {
	Status    models.MediaStatus
	RawResult string
	Adult     string
	Violence  string
	Racy      string
}

func NewModerationService() *ModerationService {
	// Check if we should use mock (no GCP credentials)
	useMock := os.Getenv("USE_MOCK_MODERATION") == "true"

	if useMock {
		log.Println("Using mock moderation service")
		return &ModerationService{useMock: true}
	}

	// Try to initialize Google Cloud Vision client
	ctx := context.Background()
	client, err := vision.NewImageAnnotatorClient(ctx)
	if err != nil {
		log.Printf("Warning: Could not create Vision client: %v. Using mock moderation.", err)
		return &ModerationService{useMock: true}
	}

	return &ModerationService{
		client:  client,
		useMock: false,
	}
}

func (s *ModerationService) ScanImage(filePath string) (*ScanResult, error) {
	if s.useMock {
		return s.mockScan(filePath)
	}

	return s.gcpScan(filePath)
}

func (s *ModerationService) mockScan(filePath string) (*ScanResult, error) {
	// Mock implementation - approve everything in dev
	log.Printf("Mock scanning file: %s", filePath)

	return &ScanResult{
		Status:    models.MediaStatusApproved,
		RawResult: `{"mock": true, "result": "approved"}`,
		Adult:     "VERY_UNLIKELY",
		Violence:  "VERY_UNLIKELY",
		Racy:      "VERY_UNLIKELY",
	}, nil
}

func (s *ModerationService) gcpScan(filePath string) (*ScanResult, error) {
	ctx := context.Background()

	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	image, err := vision.NewImageFromReader(file)
	if err != nil {
		return nil, fmt.Errorf("failed to create image: %w", err)
	}

	props, err := s.client.DetectSafeSearch(ctx, image, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to detect safe search: %w", err)
	}

	result := &ScanResult{
		Adult:    props.Adult.String(),
		Violence: props.Violence.String(),
		Racy:     props.Racy.String(),
	}

	// Serialize raw result
	rawBytes, _ := json.Marshal(map[string]string{
		"adult":    result.Adult,
		"violence": result.Violence,
		"racy":     result.Racy,
		"spoof":    props.Spoof.String(),
		"medical":  props.Medical.String(),
	})
	result.RawResult = string(rawBytes)

	// Decision logic
	result.Status = s.makeDecision(props)

	return result, nil
}

func (s *ModerationService) makeDecision(props *visionpb.SafeSearchAnnotation) models.MediaStatus {
	// Likelihood levels: UNKNOWN, VERY_UNLIKELY, UNLIKELY, POSSIBLE, LIKELY, VERY_LIKELY

	// Block if VERY_LIKELY or LIKELY explicit content
	if props.Adult >= visionpb.Likelihood_LIKELY {
		log.Printf("Blocking media: Adult content detected (%s)", props.Adult.String())
		return models.MediaStatusRejected
	}

	if props.Violence >= visionpb.Likelihood_LIKELY {
		log.Printf("Blocking media: Violence detected (%s)", props.Violence.String())
		return models.MediaStatusRejected
	}

	// Send to review if POSSIBLE
	if props.Adult >= visionpb.Likelihood_POSSIBLE ||
		props.Violence >= visionpb.Likelihood_POSSIBLE {
		log.Printf("Sending to review: Possible explicit content")
		return models.MediaStatusReview
	}

	// Approve otherwise
	return models.MediaStatusApproved
}

func (s *ModerationService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}
