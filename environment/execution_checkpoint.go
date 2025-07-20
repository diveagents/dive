package environment

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/diveagents/dive/llm"
)

// ExecutionCheckpoint represents a complete snapshot of execution state
type ExecutionCheckpoint struct {
	ID           string                 `json:"id"`
	ExecutionID  string                 `json:"execution_id"`
	WorkflowName string                 `json:"workflow_name"`
	Status       string                 `json:"status"`
	Inputs       map[string]interface{} `json:"inputs"`
	Outputs      map[string]interface{} `json:"outputs"`
	State        map[string]interface{} `json:"state"`        // Current workflow state variables
	PathStates   map[string]*PathState  `json:"path_states"`  // Current path states
	PathCounter  int                    `json:"path_counter"` // Counter for generating unique path IDs
	TotalUsage   *llm.Usage             `json:"total_usage,omitempty"`
	Error        string                 `json:"error,omitempty"`
	StartTime    time.Time              `json:"start_time"`
	EndTime      time.Time              `json:"end_time,omitempty"`
	CheckpointAt time.Time              `json:"checkpoint_at"`
}

// ExecutionCheckpointer defines simple checkpoint interface
type ExecutionCheckpointer interface {
	// SaveCheckpoint saves the current execution state
	SaveCheckpoint(ctx context.Context, checkpoint *ExecutionCheckpoint) error

	// LoadCheckpoint loads the latest checkpoint for an execution
	LoadCheckpoint(ctx context.Context, executionID string) (*ExecutionCheckpoint, error)

	// DeleteCheckpoint removes checkpoint data for an execution
	DeleteCheckpoint(ctx context.Context, executionID string) error
}

// NullCheckpointer is a no-op implementation
type NullCheckpointer struct{}

func NewNullCheckpointer() *NullCheckpointer {
	return &NullCheckpointer{}
}

func (c *NullCheckpointer) SaveCheckpoint(ctx context.Context, checkpoint *ExecutionCheckpoint) error {
	return nil
}

func (c *NullCheckpointer) LoadCheckpoint(ctx context.Context, executionID string) (*ExecutionCheckpoint, error) {
	return nil, nil
}

func (c *NullCheckpointer) DeleteCheckpoint(ctx context.Context, executionID string) error {
	return nil
}

// FileCheckpointer is a file-based implementation that persists checkpoints to disk
type FileCheckpointer struct {
	dataDir string
}

// NewFileCheckpointer creates a new file-based checkpointer
func NewFileCheckpointer(dataDir string) (*FileCheckpointer, error) {
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
		dataDir = filepath.Join(homeDir, ".dive", "executions")
	}

	// Ensure the data directory exists
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory %s: %w", dataDir, err)
	}

	return &FileCheckpointer{dataDir: dataDir}, nil
}

// SaveCheckpoint saves the execution checkpoint to disk
func (c *FileCheckpointer) SaveCheckpoint(ctx context.Context, checkpoint *ExecutionCheckpoint) error {
	executionDir := filepath.Join(c.dataDir, checkpoint.ExecutionID)
	if err := os.MkdirAll(executionDir, 0755); err != nil {
		return fmt.Errorf("failed to create execution directory: %w", err)
	}

	// Save the checkpoint as JSON
	checkpointPath := filepath.Join(executionDir, fmt.Sprintf("checkpoint-%s.json", checkpoint.ID))
	data, err := json.MarshalIndent(checkpoint, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal checkpoint: %w", err)
	}

	if err := os.WriteFile(checkpointPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write checkpoint file: %w", err)
	}

	// Update the latest checkpoint symlink
	latestPath := filepath.Join(executionDir, "latest.json")
	if err := c.updateLatestSymlink(checkpointPath, latestPath); err != nil {
		return fmt.Errorf("failed to update latest symlink: %w", err)
	}

	return nil
}

// LoadCheckpoint loads the latest checkpoint for an execution
func (c *FileCheckpointer) LoadCheckpoint(ctx context.Context, executionID string) (*ExecutionCheckpoint, error) {
	latestPath := filepath.Join(c.dataDir, executionID, "latest.json")
	
	// Check if latest checkpoint exists
	if _, err := os.Stat(latestPath); os.IsNotExist(err) {
		return nil, nil // No checkpoint found
	}

	// Read the checkpoint file
	data, err := os.ReadFile(latestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read checkpoint file: %w", err)
	}

	var checkpoint ExecutionCheckpoint
	if err := json.Unmarshal(data, &checkpoint); err != nil {
		return nil, fmt.Errorf("failed to unmarshal checkpoint: %w", err)
	}

	return &checkpoint, nil
}

// DeleteCheckpoint removes all checkpoint data for an execution
func (c *FileCheckpointer) DeleteCheckpoint(ctx context.Context, executionID string) error {
	executionDir := filepath.Join(c.dataDir, executionID)
	if err := os.RemoveAll(executionDir); err != nil {
		return fmt.Errorf("failed to delete execution directory: %w", err)
	}
	return nil
}

// ListExecutions returns a list of all executions with their latest checkpoint info
func (c *FileCheckpointer) ListExecutions(ctx context.Context) ([]*ExecutionSummary, error) {
	entries, err := os.ReadDir(c.dataDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*ExecutionSummary{}, nil // No executions directory yet
		}
		return nil, fmt.Errorf("failed to read executions directory: %w", err)
	}

	var summaries []*ExecutionSummary
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		executionID := entry.Name()
		summary, err := c.getExecutionSummary(executionID)
		if err != nil {
			// Skip executions we can't read
			continue
		}
		if summary != nil {
			summaries = append(summaries, summary)
		}
	}

	// Sort by start time (newest first)
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].StartTime.After(summaries[j].StartTime)
	})

	return summaries, nil
}

// getExecutionSummary reads the latest checkpoint and creates a summary
func (c *FileCheckpointer) getExecutionSummary(executionID string) (*ExecutionSummary, error) {
	checkpoint, err := c.LoadCheckpoint(context.Background(), executionID)
	if err != nil || checkpoint == nil {
		return nil, err
	}

	return &ExecutionSummary{
		ExecutionID:  checkpoint.ExecutionID,
		WorkflowName: checkpoint.WorkflowName,
		Status:       checkpoint.Status,
		StartTime:    checkpoint.StartTime,
		EndTime:      checkpoint.EndTime,
		Duration:     c.calculateDuration(checkpoint),
		TotalUsage:   checkpoint.TotalUsage,
		Error:        checkpoint.Error,
	}, nil
}

// calculateDuration calculates the execution duration
func (c *FileCheckpointer) calculateDuration(checkpoint *ExecutionCheckpoint) time.Duration {
	if !checkpoint.EndTime.IsZero() {
		return checkpoint.EndTime.Sub(checkpoint.StartTime)
	}
	// If still running, calculate duration from start to checkpoint time
	return checkpoint.CheckpointAt.Sub(checkpoint.StartTime)
}

// updateLatestSymlink updates the symlink to point to the latest checkpoint
func (c *FileCheckpointer) updateLatestSymlink(checkpointPath, latestPath string) error {
	// Remove existing symlink if it exists
	if _, err := os.Lstat(latestPath); err == nil {
		if err := os.Remove(latestPath); err != nil {
			return fmt.Errorf("failed to remove existing latest symlink: %w", err)
		}
	}

	// On Windows, copy the file instead of creating a symlink
	if strings.Contains(os.Getenv("OS"), "Windows") {
		data, err := os.ReadFile(checkpointPath)
		if err != nil {
			return fmt.Errorf("failed to read checkpoint for copy: %w", err)
		}
		return os.WriteFile(latestPath, data, 0644)
	}

	// Create relative symlink
	rel, err := filepath.Rel(filepath.Dir(latestPath), checkpointPath)
	if err != nil {
		return fmt.Errorf("failed to create relative path: %w", err)
	}

	return os.Symlink(rel, latestPath)
}

// ExecutionSummary provides a summary view of an execution
type ExecutionSummary struct {
	ExecutionID  string        `json:"execution_id"`
	WorkflowName string        `json:"workflow_name"`
	Status       string        `json:"status"`
	StartTime    time.Time     `json:"start_time"`
	EndTime      time.Time     `json:"end_time,omitempty"`
	Duration     time.Duration `json:"duration"`
	TotalUsage   *llm.Usage    `json:"total_usage,omitempty"`
	Error        string        `json:"error,omitempty"`
}
