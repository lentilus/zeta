package bibliography

import (
	"fmt"
	"os"
	"path/filepath"
)

type HyagrivaBib struct {
	filePath string
}

// NewHyagrivaBib creates a new HyagrivaBib instance
func NewHyagrivaBib(filePath string) *HyagrivaBib {
	return &HyagrivaBib{
		filePath: filePath,
	}
}

// Append adds new entries to the existing bibliography file
func (h *HyagrivaBib) Append(entries []Entry) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(h.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open file in append mode
	file, err := os.OpenFile(h.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Write entries
	for _, entry := range entries {
		content := formatEntry(entry)
		if _, err := file.WriteString(content); err != nil {
			return fmt.Errorf("failed to write entry: %w", err)
		}
	}

	return nil
}

// Override replaces the entire bibliography file with new entries
func (h *HyagrivaBib) Override(entries []Entry) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(h.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create or truncate file
	file, err := os.Create(h.filePath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write entries
	for _, entry := range entries {
		content := formatEntry(entry)
		if _, err := file.WriteString(content); err != nil {
			return fmt.Errorf("failed to write entry: %w", err)
		}
	}

	return nil
}

// formatEntry formats a single bibliography entry according to the specified structure
func formatEntry(entry Entry) string {
	return fmt.Sprintf("\"%s\":\n    type: Misc\n    title: \"%s\"\n    path: \"%s\"\n\n",
		entry.Target,
		entry.Title,
		entry.Path)
}
