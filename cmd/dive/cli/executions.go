package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/diveagents/dive/environment"
	"github.com/fatih/color"
	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
)

var executionsCmd = &cobra.Command{
	Use:   "executions",
	Short: "Manage workflow executions",
	Long:  "Commands for managing and viewing workflow execution history",
}

var (
	statusFilterFlag string
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all workflow executions",
	Long:  "List all workflow executions with their status and basic information",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate status filter using utility from common.go
		if err := validateExecutionStatus(statusFilterFlag); err != nil {
			return err
		}

		return listExecutions(statusFilterFlag)
	},
}

func listExecutions(statusFilter string) error {
	// Use database path utility from common.go
	databasePath, err := getDiveDatabaseDir()
	if err != nil {
		return fmt.Errorf("error getting database path: %v", err)
	}

	checkpointer, err := environment.NewFileCheckpointer(databasePath)
	if err != nil {
		return fmt.Errorf("error creating checkpointer: %v", err)
	}

	ctx := context.Background()
	executions, err := checkpointer.ListExecutions(ctx)
	if err != nil {
		return fmt.Errorf("error listing executions: %v", err)
	}

	// Filter by status if specified
	if statusFilter != "" {
		var filteredExecutions []*environment.ExecutionSummary
		for _, exec := range executions {
			if exec.Status == statusFilter {
				filteredExecutions = append(filteredExecutions, exec)
			}
		}
		executions = filteredExecutions
	}

	if len(executions) == 0 {
		if statusFilter != "" {
			fmt.Printf("%s No executions found with status '%s'.\n", color.New(color.FgCyan).Sprint("ℹ️"), statusFilter)
		} else {
			fmt.Printf("%s No executions found.\n", color.New(color.FgCyan).Sprint("ℹ️"))
		}
		return nil
	}

	// Print header with improved styling
	if len(executions) == 1 {
		fmt.Printf("\nFound %d execution\n\n", len(executions))
	} else {
		fmt.Printf("\nFound %d executions\n\n", len(executions))
	}

	// Create table using tablewriter for proper alignment
	table := tablewriter.NewTable(os.Stdout)
	table.Header("EXECUTION ID", "WORKFLOW", "STATUS", "START TIME", "DURATION", "ERROR")

	// Prepare data for bulk insert
	var tableData [][]any
	for _, exec := range executions {
		duration := formatDuration(exec.Duration)
		startTime := exec.StartTime.Format("2006-01-02 15:04:05")

		errorText := ""
		if exec.Error != "" {
			errorText = truncateString(exec.Error, 40)
		}

		// Use formatExecutionStatus utility from common.go for enhanced status display
		formattedStatus := formatExecutionStatus(exec.Status)

		tableData = append(tableData, []any{
			exec.ExecutionID,
			truncateString(exec.WorkflowName, 20),
			formattedStatus,
			startTime,
			duration,
			errorText,
		})
	}

	table.Bulk(tableData)
	table.Render()

	fmt.Println()

	return nil
}

// formatDuration formats a duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	if d < time.Hour {
		return fmt.Sprintf("%.1fm", d.Minutes())
	}
	return fmt.Sprintf("%.1fh", d.Hours())
}

// truncateString truncates a string to the specified length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

var resumeCmd = &cobra.Command{
	Use:   "resume <execution-id>",
	Short: "Resume a failed workflow execution",
	Long:  "Resume a previously failed workflow execution from the point of failure using the latest checkpoint",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		executionID := args[0]
		return resumeExecution(executionID)
	},
}

// resumeExecution resumes a failed execution
func resumeExecution(executionID string) error {
	// Get database path
	databasePath, err := getDiveDatabaseDir()
	if err != nil {
		return fmt.Errorf("error getting database path: %v", err)
	}

	// Create checkpointer to load execution data
	checkpointer, err := environment.NewFileCheckpointer(databasePath)
	if err != nil {
		return fmt.Errorf("error creating checkpointer: %v", err)
	}

	ctx := context.Background()

	// Load the checkpoint to verify execution exists and is failed
	checkpoint, err := checkpointer.LoadCheckpoint(ctx, executionID)
	if err != nil {
		return fmt.Errorf("error loading checkpoint for execution %s: %v", executionID, err)
	}

	if checkpoint == nil {
		return fmt.Errorf("❌ Execution %s not found", executionID)
	}

	// Check if execution is in a resumable state
	if checkpoint.Status != "failed" {
		return fmt.Errorf("❌ Cannot resume execution %s: status is %s (only failed executions can be resumed)",
			executionID, checkpoint.Status)
	}

	// Confirm the resume action
	fmt.Printf("📋 Execution Details:\n")
	fmt.Printf("   ID: %s\n", executionID)
	fmt.Printf("   Workflow: %s\n", checkpoint.WorkflowName)
	fmt.Printf("   Status: %s\n", formatExecutionStatus(checkpoint.Status))
	fmt.Printf("   Failed at: %s\n", checkpoint.EndTime.Format("2006-01-02 15:04:05"))
	if checkpoint.Error != "" {
		fmt.Printf("   Error: %s\n", color.New(color.FgYellow).Sprint(checkpoint.Error))
	}
	fmt.Println()

	if !ConfirmAction("resume", fmt.Sprintf("execution %s", executionID)) {
		fmt.Printf("%s Resume operation cancelled.\n", color.New(color.FgCyan).Sprint("ℹ️"))
		return nil
	}

	fmt.Printf("%s Resuming execution %s...\n", color.New(color.FgCyan).Sprint("🔄"), executionID)

	// Load the workflow from config (we need the workflow definition to resume)
	// For now, we'll show an error message about needing to provide workflow path
	// This could be enhanced later to store workflow definition in checkpoint

	return fmt.Errorf("❌ Resume functionality integrated with run command.\n\n"+
		"💡 To resume this execution, use the run command with --resume flag:\n"+
		"   dive run --resume %s <workflow-file>", executionID)
}

func init() {
	rootCmd.AddCommand(executionsCmd)
	executionsCmd.AddCommand(listCmd)
	executionsCmd.AddCommand(resumeCmd)

	// Add flags using the utilities patterns from common.go
	listCmd.Flags().StringVarP(&statusFilterFlag, "status", "s", "", "Filter by execution status (pending, running, completed, failed)")
}
