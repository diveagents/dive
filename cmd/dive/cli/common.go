package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// getDiveConfigDir returns the dive configuration directory, creating it if it doesn't exist
func getDiveConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	diveDir := filepath.Join(homeDir, ".dive")
	if err := os.MkdirAll(diveDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create dive config directory: %w", err)
	}

	return diveDir, nil
}

// getDiveDatabaseDir returns the database directory
func getDiveDatabaseDir() (string, error) {
	diveDir, err := getDiveConfigDir()
	if err != nil {
		return "", fmt.Errorf("error getting dive config directory: %v", err)
	}
	return filepath.Join(diveDir, "executions"), nil
}

// validateExecutionStatus validates and suggests valid status values
func validateExecutionStatus(status string) error {
	if status == "" {
		return nil
	}

	validStatuses := []string{"pending", "running", "completed", "failed"}
	for _, validStatus := range validStatuses {
		if status == validStatus {
			return nil
		}
	}

	return fmt.Errorf("❌ Invalid status '%s'\n\n💡 Valid status values: %s", status, strings.Join(validStatuses, ", "))
}

// ConfirmAction prompts the user for confirmation with a standardized message
func ConfirmAction(action, target string) bool {
	fmt.Printf("❓ Are you sure you want to %s %s? [y/N]: ", action, target)
	var response string
	fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// formatExecutionStatus returns a consistently formatted status string with icons and colors
func formatExecutionStatus(status string) string {
	switch strings.ToLower(status) {
	case "completed":
		return successStyle.Sprint("✓ " + status)
	case "failed":
		return errorStyle.Sprint("✗ " + status)
	case "running":
		return runningStyle.Sprint("⚠ " + status)
	case "pending":
		return infoStyle.Sprint("⏳ " + status)
	default:
		return infoStyle.Sprint("• " + status)
	}
}
