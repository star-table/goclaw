package service

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/smallnest/goclaw/console/internal/model"
)

const (
	// DefaultHeartbeatEvery default interval (6 hours)
	DefaultHeartbeatEvery = "6h"
	// DefaultHeartbeatTarget default target
	DefaultHeartbeatTarget = "main"
	// HeartbeatTargetLast special target to dispatch to last channel
	HeartbeatTargetLast = "last"
)

// HeartbeatService manages heartbeat configuration with persistence
type HeartbeatService struct {
	config  *model.HeartbeatConfig
	baseDir string
	mu      sync.RWMutex
}

// NewHeartbeatService creates a new heartbeat service with persistence
func NewHeartbeatService(baseDir string) *HeartbeatService {
	s := &HeartbeatService{
		baseDir: baseDir,
	}

	// Try to load existing config, otherwise use default
	if err := s.loadConfig(); err != nil {
		s.config = model.DefaultHeartbeatConfig()
		// Save default config
		_ = s.saveConfig()
	}

	return s
}

// GetConfig returns heartbeat configuration
func (s *HeartbeatService) GetConfig() *model.HeartbeatConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Return a copy to prevent external modification
	configCopy := *s.config
	return &configCopy
}

// UpdateConfig updates heartbeat configuration and persists it
func (s *HeartbeatService) UpdateConfig(config *model.HeartbeatConfig) *model.HeartbeatConfig {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.config = config
	// Persist to disk
	if err := s.saveConfig(); err != nil {
		// Log error but don't fail the update
		fmt.Printf("Failed to save heartbeat config: %v\n", err)
	}

	// Return a copy
	configCopy := *s.config
	return &configCopy
}

// loadConfig loads heartbeat config from disk
func (s *HeartbeatService) loadConfig() error {
	if s.baseDir == "" {
		return fmt.Errorf("baseDir not set")
	}

	configFile := filepath.Join(s.baseDir, "heartbeat.json")
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, use default
			s.config = model.DefaultHeartbeatConfig()
			return nil
		}
		return err
	}

	var config model.HeartbeatConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	s.config = &config
	return nil
}

// saveConfig saves heartbeat config to disk
func (s *HeartbeatService) saveConfig() error {
	if s.baseDir == "" {
		return fmt.Errorf("baseDir not set")
	}

	// Ensure directory exists
	if err := os.MkdirAll(s.baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	configFile := filepath.Join(s.baseDir, "heartbeat.json")
	data, err := json.MarshalIndent(s.config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Direct write (temp file + rename may be blocked by security policies)
	if err := os.WriteFile(configFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// ParseHeartbeatEvery parses interval string (e.g. "30m", "1h", "6h") to total seconds
// Matches Python implementation: parse_heartbeat_every
func ParseHeartbeatEvery(every string) int {
	every = trimSpace(every)
	if every == "" {
		return 30 * 60 // default 30 minutes
	}

	// Pattern: ^(?:(?P<hours>\d+)h)?(?:(?P<minutes>\d+)m)?(?:(?P<seconds>\d+)s)?$
	pattern := regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)
	matches := pattern.FindStringSubmatch(every)

	if matches == nil {
		// Invalid format, return default
		return 30 * 60
	}

	hours := 0
	minutes := 0
	seconds := 0

	if matches[1] != "" {
		hours, _ = strconv.Atoi(matches[1])
	}
	if matches[2] != "" {
		minutes, _ = strconv.Atoi(matches[2])
	}
	if matches[3] != "" {
		seconds, _ = strconv.Atoi(matches[3])
	}

	total := hours*3600 + minutes*60 + seconds
	if total <= 0 {
		return 30 * 60
	}

	return total
}

// InActiveHours checks if current time is within active hours
// Matches Python implementation: _in_active_hours
func InActiveHours(activeHours *model.ActiveHoursConfig) bool {
	if activeHours == nil || activeHours.Start == "" || activeHours.End == "" {
		return true
	}

	now := time.Now()
	startTime, err1 := parseTime(activeHours.Start)
	endTime, err2 := parseTime(activeHours.End)

	if err1 != nil || err2 != nil {
		return true
	}

	currentTime := time.Date(0, 1, 1, now.Hour(), now.Minute(), 0, 0, time.Local)

	if startTime.Before(endTime) || startTime.Equal(endTime) {
		return (currentTime.Equal(startTime) || currentTime.After(startTime)) &&
			(currentTime.Equal(endTime) || currentTime.Before(endTime))
	}

	// Crosses midnight (e.g., 22:00-08:00)
	return currentTime.After(startTime) || currentTime.Before(endTime)
}

// parseTime parses time string like "08:00" or "8:00"
func parseTime(t string) (time.Time, error) {
	// Try different formats
	formats := []string{"15:04", "3:04 PM", "3:04PM", "3:04"}
	for _, format := range formats {
		if parsed, err := time.Parse(format, trimSpace(t)); err == nil {
			return parsed, nil
		}
	}
	return time.Time{}, fmt.Errorf("unable to parse time: %s", t)
}

// trimSpace removes whitespace from string
func trimSpace(s string) string {
	return regexp.MustCompile(`^\s+|\s+$`).ReplaceAllString(s, "")
}
