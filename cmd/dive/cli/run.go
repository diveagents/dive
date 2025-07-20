package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/diveagents/dive/config"
	"github.com/diveagents/dive/environment"
	"github.com/diveagents/dive/slogger"
	"github.com/spf13/cobra"
)

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func runWorkflow(filePath string, workflowName string, logLevel slogger.LogLevel, resumeExecutionID string) error {
	startTime := time.Now()

	ctx := context.Background()
	logger := slogger.New(logLevel)

	// Check if path is a directory or file
	fi, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("❌ Cannot access workflow path '%s': %v", filePath, err)
	}

	configDir := filePath
	basePath := ""

	// If a single file is provided, copy it to a temporary directory
	if !fi.IsDir() {
		// Create temp directory
		tempDir, err := os.MkdirTemp("", "dive-workflow-*")
		if err != nil {
			return fmt.Errorf("❌ Failed to create temporary directory: %v", err)
		}
		defer os.RemoveAll(tempDir)

		// Copy the file to temp directory
		fileName := filepath.Base(filePath)
		destPath := filepath.Join(tempDir, fileName)
		if err := copyFile(filePath, destPath); err != nil {
			return fmt.Errorf("❌ Failed to copy workflow file: %v", err)
		}

		configDir = tempDir
		basePath = filepath.Dir(filePath)
	}

	// Create build options
	buildOptions := []config.BuildOption{
		config.WithLogger(logger),
		config.WithBasePath(basePath),
	}

	// Load and build environment from directory
	env, err := config.LoadDirectory(configDir, buildOptions...)
	if err != nil {
		return fmt.Errorf("❌ Failed to load workflow configuration: %v\n\n💡 Check that your YAML syntax is correct and all required fields are present", err)
	}
	if err := env.Start(ctx); err != nil {
		return fmt.Errorf("error starting environment: %v", err)
	}
	defer env.Stop(ctx)

	// Determine workflow name
	if workflowName == "" {
		workflows := env.Workflows()
		if len(workflows) == 0 {
			return fmt.Errorf("❌ No workflows found in %s", filePath)
		}
		if len(workflows) == 1 {
			workflowName = workflows[0].Name()
		} else {
			var workflowNames []string
			for _, w := range workflows {
				workflowNames = append(workflowNames, w.Name())
			}
			return fmt.Errorf("❌ Multiple workflows found, specify one using --workflow: %v", workflowNames)
		}
	}

	wf, err := env.GetWorkflow(workflowName)
	if err != nil {
		workflows := env.Workflows()
		var workflowNames []string
		for _, wf := range workflows {
			workflowNames = append(workflowNames, wf.Name())
		}
		if len(workflowNames) > 0 {
			return fmt.Errorf("❌ Workflow '%s' not found\n\n💡 Available workflows: %v", workflowName, workflowNames)
		}
		return fmt.Errorf("❌ Workflow '%s' not found: %v", workflowName, err)
	}

	formatter := NewWorkflowFormatter()
	formatter.PrintWorkflowHeader(wf, getUserVariables())

	// Create file-based checkpointer for persistent execution state
	checkpointer, err := environment.NewFileCheckpointer("")
	if err != nil {
		duration := time.Since(startTime)
		formatter.PrintWorkflowError(err, duration)
		return fmt.Errorf("error creating checkpointer: %v", err)
	}

	var execution *environment.Execution

	// Handle resume vs new execution
	if resumeExecutionID != "" {
		// Create execution with the specific execution ID for resuming
		execution, err = environment.NewExecution(environment.ExecutionOptions{
			Workflow:     wf,
			Environment:  env,
			Inputs:       getUserVariables(),
			Logger:       logger,
			Formatter:    formatter,
			Checkpointer: checkpointer,
			ExecutionID:  resumeExecutionID,
		})
		if err != nil {
			duration := time.Since(startTime)
			formatter.PrintWorkflowError(err, duration)
			return fmt.Errorf("error creating execution for resume: %v", err)
		}

		fmt.Printf("🔄 Resuming execution %s...\n", resumeExecutionID)

		// Use ResumeFromFailure instead of Run
		if err := execution.ResumeFromFailure(ctx); err != nil {
			duration := time.Since(startTime)
			formatter.PrintWorkflowError(err, duration)
			formatter.PrintExecutionNextSteps(execution.ID())
			return fmt.Errorf("error resuming workflow: %v", err)
		}
	} else {
		// Create execution with persistent checkpoint-based model
		execution, err = environment.NewExecution(environment.ExecutionOptions{
			Workflow:     wf,
			Environment:  env,
			Inputs:       getUserVariables(),
			Logger:       logger,
			Formatter:    formatter,
			Checkpointer: checkpointer,
		})
		if err != nil {
			duration := time.Since(startTime)
			formatter.PrintWorkflowError(err, duration)
			return fmt.Errorf("error creating execution: %v", err)
		}

		formatter.PrintExecutionID(execution.ID())

		if err := execution.Run(ctx); err != nil {
			duration := time.Since(startTime)
			formatter.PrintWorkflowError(err, duration)
			formatter.PrintExecutionNextSteps(execution.ID())
			return fmt.Errorf("error running workflow: %v", err)
		}
	}

	duration := time.Since(startTime)
	formatter.PrintWorkflowComplete(duration)
	return nil
}

var runCmd = &cobra.Command{
	Use:   "run [file or directory]",
	Short: "Run a workflow",
	Long:  "Run a workflow with automatic checkpoint-based state management, or resume a failed execution",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		workflowName, err := cmd.Flags().GetString("workflow")
		if err != nil {
			fmt.Println(workflowError.Sprint(err))
			os.Exit(1)
		}
		resumeExecutionID, err := cmd.Flags().GetString("resume")
		if err != nil {
			fmt.Println(workflowError.Sprint(err))
			os.Exit(1)
		}
		if err := runWorkflow(filePath, workflowName, getLogLevel(), resumeExecutionID); err != nil {
			fmt.Println(workflowError.Sprint(err))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringP("workflow", "w", "", "Name of the workflow to run")
	runCmd.Flags().StringP("resume", "r", "", "Resume a failed execution by providing its execution ID")
}
