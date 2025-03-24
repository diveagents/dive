package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/getstingrai/dive/config"
	"github.com/getstingrai/dive/slogger"
	"github.com/spf13/cobra"
)

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("error opening source file: %v", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("error creating destination file: %v", err)
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("error copying file: %v", err)
	}
	return nil
}

func runWorkflow(path, workflowName string, logLevel slogger.LogLevel) error {
	ctx := context.Background()

	// Check if path is a directory or file
	fi, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("error accessing path: %v", err)
	}

	configDir := path

	// If a single file is provided, copy it to a temporary directory
	// and use that as the config directory.
	if !fi.IsDir() {
		tmpDir, err := os.MkdirTemp("", "dive-config-*")
		if err != nil {
			return fmt.Errorf("error creating temp directory: %v", err)
		}
		defer os.RemoveAll(tmpDir)

		dst := filepath.Join(tmpDir, filepath.Base(path))
		if err := copyFile(path, dst); err != nil {
			return err
		}
		configDir = tmpDir
	}

	buildOpts := []config.BuildOption{}
	if logLevel != 0 {
		logger := slogger.New(logLevel)
		buildOpts = append(buildOpts, config.WithLogger(logger))
	}
	env, err := config.LoadDirectory(configDir, buildOpts...)
	if err != nil {
		return fmt.Errorf("error loading environment: %v", err)
	}
	if err := env.Start(ctx); err != nil {
		return fmt.Errorf("error starting environment: %v", err)
	}
	defer env.Stop(ctx)

	if workflowName == "" {
		workflows := env.Workflows()
		if len(workflows) != 1 {
			return fmt.Errorf("you must specify a workflow name")
		}
		workflowName = workflows[0].Name()
	}

	execution, err := env.ExecuteWorkflow(ctx, workflowName, getUserVariables())
	if err != nil {
		return fmt.Errorf("error executing workflow: %v", err)
	}
	if err := execution.Wait(); err != nil {
		return fmt.Errorf("error waiting for workflow: %v", err)
	}

	// // Monitor execution events
	// for event := range execution.Events() {
	// 	switch e := event.(type) {
	// 	case *dive.StepStartEvent:
	// 		fmt.Printf("\nStarting step: %s\n", boldStyle.Sprint(e.Name))
	// 	case *dive.StepCompleteEvent:
	// 		fmt.Printf("\nCompleted step: %s\n", boldStyle.Sprint(e.Name))
	// 		if e.Result != nil {
	// 			fmt.Printf("\nResult:\n%s\n", e.Result)
	// 		}
	// 	case *dive.StepErrorEvent:
	// 		fmt.Printf("\nStep failed: %s\nError: %v\n", errorStyle.Sprint(e.Name), e.Error)
	// 	}
	// }

	return nil
}

var runCmd = &cobra.Command{
	Use:   "run [file or directory]",
	Short: "Run a workflow",
	Long:  "Run a workflow",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filePath := args[0]
		workflowName, err := cmd.Flags().GetString("workflow")
		if err != nil {
			fmt.Println(errorStyle.Sprint(err))
			os.Exit(1)
		}
		if err := runWorkflow(filePath, workflowName, getLogLevel()); err != nil {
			fmt.Println(errorStyle.Sprint(err))
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	runCmd.Flags().StringP("workflow", "w", "", "Name of the workflow to run")
}
