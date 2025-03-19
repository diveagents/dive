package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/getstingrai/dive/config"
	"github.com/getstingrai/dive/slogger"
	"github.com/getstingrai/dive/workflow"
)

var (
	errorStyle = color.New(color.FgRed)
)

func fatal(msg string, args ...interface{}) {
	fmt.Printf(errorStyle.Sprint(msg)+"\n", args...)
	os.Exit(1)
}

func main() {
	var logLevel, varsFlag, outDir, workflowName string
	flag.StringVar(&logLevel, "log-level", "debug", "Log level (debug, info, warn, error)")
	flag.StringVar(&varsFlag, "vars", "", "Comma-separated list of variables in format key=value")
	flag.StringVar(&outDir, "output", "", "Output directory for task results")
	flag.StringVar(&workflowName, "workflow", "", "Workflow name")
	flag.Parse()

	if flag.NArg() == 0 {
		fatal("Error: file path is required")
	}

	filePath := flag.Arg(0)

	vars := map[string]any{}
	if varsFlag != "" {
		varPairs := strings.Split(varsFlag, ",")
		for _, pair := range varPairs {
			parts := strings.SplitN(pair, "=", 2)
			if len(parts) != 2 {
				fatal("Error: invalid variable format: %s", pair)
			}
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			vars[key] = value
		}
	}

	ctx := context.Background()

	env, err := config.LoadDirectory(filePath, config.WithLogger(slogger.New(slogger.LevelDebug)))
	if err != nil {
		fatal(err.Error())
	}

	var workflow *workflow.Workflow
	if workflowName != "" {
		workflow, err = env.GetWorkflow(workflowName)
		if err != nil {
			fatal(err.Error())
		}
	} else {
		workflows := env.Workflows()
		if len(workflows) != 1 {
			fatal("You must specify a workflow name")
		}
		workflow = workflows[0]
	}

	execution, err := env.StartWorkflow(ctx, workflow, vars)
	if err != nil {
		fatal(err.Error())
	}

	if err := execution.Wait(); err != nil {
		fatal(err.Error())
	}

	if outDir != "" {
		if err := os.MkdirAll(outDir, 0755); err != nil {
			fatal("Error: failed to create output directory: %s", err)
		}
	}
}
