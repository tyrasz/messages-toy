package services

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// VideoMetadata contains extracted video information
type VideoMetadata struct {
	Duration      int    // Duration in seconds
	Width         int    // Video width
	Height        int    // Video height
	ThumbnailPath string // Path to generated thumbnail
}

// AudioMetadata contains extracted audio information
type AudioMetadata struct {
	Duration int // Duration in seconds
}

// VideoService handles video processing operations
type VideoService struct {
	ffprobePath string
	ffmpegPath  string
	outputDir   string
}

// NewVideoService creates a new VideoService
func NewVideoService() *VideoService {
	// Look for ffprobe and ffmpeg in PATH
	ffprobePath, _ := exec.LookPath("ffprobe")
	ffmpegPath, _ := exec.LookPath("ffmpeg")

	return &VideoService{
		ffprobePath: ffprobePath,
		ffmpegPath:  ffmpegPath,
		outputDir:   "./uploads/thumbnails",
	}
}

// IsAvailable checks if ffprobe and ffmpeg are installed
func (s *VideoService) IsAvailable() bool {
	return s.ffprobePath != "" && s.ffmpegPath != ""
}

// ExtractVideoMetadata extracts metadata from a video file
func (s *VideoService) ExtractVideoMetadata(videoPath string) (*VideoMetadata, error) {
	if s.ffprobePath == "" {
		return nil, fmt.Errorf("ffprobe not found in PATH")
	}

	// Get video info using ffprobe
	cmd := exec.Command(s.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probeData struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(output, &probeData); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	metadata := &VideoMetadata{}

	// Parse duration
	if probeData.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			metadata.Duration = int(dur)
		}
	}

	// Find video stream dimensions
	for _, stream := range probeData.Streams {
		if stream.CodecType == "video" {
			metadata.Width = stream.Width
			metadata.Height = stream.Height
			break
		}
	}

	return metadata, nil
}

// GenerateThumbnail creates a thumbnail from a video file
func (s *VideoService) GenerateThumbnail(videoPath, outputName string) (string, error) {
	if s.ffmpegPath == "" {
		return "", fmt.Errorf("ffmpeg not found in PATH")
	}

	// Ensure output directory exists
	if err := os.MkdirAll(s.outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// Generate thumbnail filename
	thumbnailName := strings.TrimSuffix(outputName, filepath.Ext(outputName)) + ".jpg"
	thumbnailPath := filepath.Join(s.outputDir, thumbnailName)

	// Extract thumbnail at 1 second (or start if video is shorter)
	cmd := exec.Command(s.ffmpegPath,
		"-i", videoPath,
		"-ss", "00:00:01",    // Seek to 1 second
		"-vframes", "1",       // Extract 1 frame
		"-vf", "scale=320:-1", // Scale width to 320, maintain aspect ratio
		"-q:v", "5",           // Quality (2-31, lower is better)
		"-y",                  // Overwrite output
		thumbnailPath,
	)

	if err := cmd.Run(); err != nil {
		// Try extracting from beginning if seeking fails
		cmd = exec.Command(s.ffmpegPath,
			"-i", videoPath,
			"-vframes", "1",
			"-vf", "scale=320:-1",
			"-q:v", "5",
			"-y",
			thumbnailPath,
		)
		if err := cmd.Run(); err != nil {
			return "", fmt.Errorf("failed to generate thumbnail: %w", err)
		}
	}

	return thumbnailPath, nil
}

// ExtractAudioMetadata extracts metadata from an audio file
func (s *VideoService) ExtractAudioMetadata(audioPath string) (*AudioMetadata, error) {
	if s.ffprobePath == "" {
		return nil, fmt.Errorf("ffprobe not found in PATH")
	}

	cmd := exec.Command(s.ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		audioPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probeData struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probeData); err != nil {
		return nil, fmt.Errorf("failed to parse ffprobe output: %w", err)
	}

	metadata := &AudioMetadata{}

	if probeData.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			metadata.Duration = int(dur)
		}
	}

	return metadata, nil
}

// ProcessVideo extracts metadata and generates thumbnail for a video
func (s *VideoService) ProcessVideo(videoPath, filename string) (*VideoMetadata, error) {
	metadata, err := s.ExtractVideoMetadata(videoPath)
	if err != nil {
		return nil, err
	}

	thumbnailPath, err := s.GenerateThumbnail(videoPath, filename)
	if err != nil {
		// Return metadata without thumbnail if generation fails
		return metadata, nil
	}

	metadata.ThumbnailPath = thumbnailPath
	return metadata, nil
}
