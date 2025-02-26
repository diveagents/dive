<p align="center">
  <img src="https://getstingrai-public.s3.us-east-1.amazonaws.com/static/images/dive/dive-logo-2025-02-25-1024.jpg" width="200" height="200">
  <h1 align="center">
    <a href="https://github.com/getstingrai/dive">Dive - AI Agent Framework</a>
  </h1>
</p>

## Introduction

Dive is a powerful, flexible Go framework for building AI agent systems that
actually get things done. Whether you need a single specialized agent or a
collaborative team of AI workers, Dive makes it easy to create, orchestrate, and
manage AI agents powered by leading LLM providers.

This approach enables complex agent behaviors while maintaining a simple,
intuitive API for developers.

Dive can easily be embedded into existing Go applications, or run as a
standalone process.

## Project Status

**⚠️ Early Development Stage ⚠️**

Dive is currently in its early development stages. While the core functionality
is in place, the project is still evolving rapidly.

- **Not recommended for production use** at this time
- **Breaking changes will happen** as the API matures
- **Feedback is highly valued** on concepts, APIs, and usability

We welcome your input! Please reach out in
[GitHub Discussions](https://github.com/getstingrai/dive/discussions) with
questions, suggestions, or feedback.

## Features

* **Flexible Agent Architecture**: Create specialized agents with different roles, capabilities, and access to tools
* **Team Collaboration**: Build teams of agents that work together, with supervisors that can delegate tasks
* **Task Management**: Define, assign, and track tasks with dependencies, timeouts, and expected outputs
* **Multi-Provider Support**: Use your preferred LLM provider (Anthropic, OpenAI, Groq) with a unified interface
* **Powerful Tool System**: Extend agent capabilities with tools like web search, document retrieval, and more
* **Declarative Configuration**: Define agents and teams using simple YAML files or programmatically in Go
* **Robust Error Handling**: Built-in retry mechanisms, timeouts, and error recovery for reliable operation
* **Comprehensive Logging**: Detailed logging of agent activities, decisions, and task progress

## Installation

Getting started with Dive is simple. You can install it using Go's standard package management:

```bash
go get github.com/getstingrai/dive
```

### Prerequisites

- Go 1.20 or higher
- API keys for the LLM providers you plan to use (Anthropic, OpenAI, Groq, etc.)
- API keys for any external tools you plan to use (Google Search, Firecrawl, etc.)

### Environment Setup

Set up your environment variables for the LLM providers and tools:

```bash
# LLM Provider API Keys
export ANTHROPIC_API_KEY="your-anthropic-api-key"
export OPENAI_API_KEY="your-openai-api-key"
export GROQ_API_KEY="your-groq-api-key"

# Tool API Keys
export GOOGLE_SEARCH_API_KEY="your-google-search-api-key"
export GOOGLE_SEARCH_CX="your-google-search-cx"
export FIRECRAWL_API_KEY="your-firecrawl-api-key"
```

## Quick Start

Get started with Dive in just a few lines of code:

```go
package main

import (
	"context"
	"log"

	"github.com/getstingrai/dive"
	"github.com/getstingrai/dive/providers/anthropic"
	"github.com/getstingrai/dive/tools"
	"github.com/getstingrai/dive/tools/google"
)

func main() {
	ctx := context.Background()

	// Create a Google Search API client
	googleClient, err := google.New()
	if err != nil {
		log.Fatal(err)
	}

	// Create a new agent
	agent := dive.NewAgent(dive.AgentOptions{
		Name: "researcher",
		Role: &dive.Role{
			Description: "Research Assistant",
			AcceptsWork: []string{"research"},
		},
		LLM:          anthropic.New(),
		CacheControl: "ephemeral",
		Tools:        []dive.Tool{tools.NewGoogleSearch(googleClient)},
	})

	// Start the agent
	if err := agent.Start(ctx); err != nil {
		log.Fatal(err)
	}
	defer agent.Stop(ctx)

	// Create and assign a task
	task := dive.NewTask(dive.TaskOptions{
		Description: "Research the history of AI and summarize in 3 paragraphs",
	})
	
	promise, err := agent.Work(ctx, task)
	if err != nil {
		log.Fatal(err)
	}
	
	// Wait for the result
	result, err := promise.Get(ctx)
	if err != nil {
		log.Fatal(err)
	}
	
	log.Printf("Task result: %s", result.Output)
}
```

Or use the YAML runner for a declarative approach:

```bash
go run cmd/yaml_runner/main.go -file=examples/research_team.yaml
```

### Tasks

Tasks are the fundamental units of work in Dive. Each task represents a discrete
piece of work that an agent needs to complete, with clear inputs, expected
outputs, and constraints.

Tasks can be simple one-off requests or part of a complex workflow with
dependencies on other tasks. The framework handles task scheduling, execution,
and result management, allowing you to focus on defining what needs to be done
rather than how to manage the execution.

Key features of tasks include:

* **Rich Descriptions**: Provide detailed instructions and context for the agent
* **Expected Outputs**: Define what a successful result should look like
* **Dependencies**: Create task workflows where tasks depend on the results of previous tasks
* **Timeouts**: Set maximum execution times to prevent runaway processes
* **Iteration Limits**: Control how many attempts an agent can make to complete a task
* **Output Formatting**: Specify how results should be formatted (text, JSON, etc.)
* **File Output**: Save task results directly to files

Creating a task is as simple as:

```go
task := dive.NewTask(dive.TaskOptions{
    Name:           "market-research",
    Description:    "Research the current market trends for electric vehicles",
    ExpectedOutput: "A 500-word summary with 3 key insights",
    Timeout:        5 * time.Minute,
})
```

## Core Concepts

### Agents

Agents are the intelligent workers in the Dive framework. Each agent is powered
by a Large Language Model (LLM) and can be specialized for different roles and
tasks. Agents can work independently or as part of a team, and they can use
tools to interact with the outside world.

Key aspects of agents include:

* **Independent Operation**: Each agent runs in its own goroutine with a dedicated run loop
* **Role-Based Specialization**: Define specific roles and responsibilities for each agent
* **Tool Access**: Equip agents with the tools they need for their specific tasks
* **State Management**: Agents maintain their own state and context across interactions

Creating an agent is straightforward:

```go
agent := dive.NewAgent(dive.AgentOptions{
    Name:         "analyst",
    Role:         &dive.Role{Description: "Financial Analyst"},
    LLM:          anthropic.New(),
    Tools:        []dive.Tool{},
})
```

### Teams

Teams allow multiple agents to collaborate on complex tasks. A team consists of
a group of agents, potentially with different roles and capabilities, working
together toward a common goal.

Teams can have hierarchical structures, with supervisor agents delegating tasks
to subordinate agents based on their specialties.

Key features of teams include:

* **Hierarchical Structure**: Create teams with supervisors and subordinates
* **Task Assignment**: Supervisors can assign tasks to the most appropriate agent
* **Shared Context**: Team members can share context and build on each other's work
* **Parallel Execution**: Multiple agents can work on different tasks simultaneously
* **Coordinated Workflows**: Create complex workflows with dependencies between tasks

Creating a team programmatically:

```go
team, err := dive.NewTeam(dive.TeamOptions{
    Name:        "research-team",
    Description: "A team for market research and analysis",
    Agents:      []dive.Agent{supervisor, researcher, analyst},
})
```

Or define a team declaratively in YAML:

```yaml
name: 'Research Team'
description: 'A team for market research and analysis'

agents:
  - name: 'Supervisor'
    role:
      description: 'Research Team Lead'
      is_supervisor: true
      subordinates: ['Researcher', 'Analyst']
  
  - name: 'Researcher'
    role:
      description: 'Market Researcher'
    tools: ['google_search', 'web_scraper']
  
  - name: 'Analyst'
    role:
      description: 'Data Analyst'
    tools: ['calculator', 'data_fetcher']
```

### Execution Model

Dive embraces the actor model of concurrency, providing a robust foundation for
building and scaling complex agent systems. This execution model enables agents
to work concurrently on different tasks while maintaining strict isolation
between them.

The actor-inspired design is particularly well-suited for AI agent systems,
where each agent needs to maintain its own context, make independent decisions,
and collaborate with others through well-defined interfaces.

### Events

**Disclaimer:** This is not fully implemented yet.

Events provide a way to notify agents about changes or occurrences that might
require their attention. Events can be used to trigger agent actions, provide
new information, or coordinate activities between agents.

Events consist of a name, description, and optional parameters. They can be sent
to individual agents or broadcast to an entire team.

Examples of events include:

* Notifying agents of new data becoming available
* Alerting agents to changes in the environment
* Triggering periodic tasks or reviews
* Coordinating activities between multiple agents

Sending an event to an agent:

```go
event := &dive.Event{
    Name:        "new_data_available",
    Description: "New market data is available for analysis",
    Parameters: map[string]any{
        "data_source": "quarterly_reports",
        "timestamp":   time.Now(),
    },
}

err := agent.Event(ctx, event)
```

### Roles

Roles define an agent's responsibilities, capabilities, and position within a
team. A role includes a description of the agent's purpose, whether it's a
supervisor, what types of work it accepts, and more.

Roles help agents understand their purpose and constraints, guiding their
decision-making and actions. They also help with task routing, ensuring that
tasks are assigned to the most appropriate agent.

Key components of a role include:

* **Description**: A natural language description of the agent's role
* **Supervisor Status**: Whether the agent can delegate tasks to others
* **Subordinates**: Which agents report to this agent (if a supervisor)
* **Work Types**: What kinds of tasks the agent can accept
* **Event Types**: What kinds of events the agent can handle

Defining a role:

```go
role := &dive.Role{
    Description:   "You are a data scientist specializing in market trend analysis",
    IsSupervisor:  false,
    AcceptsChats:  true,
    AcceptsEvents: []string{"data_update", "analysis_request"},
    AcceptsWork:   []string{"data_analysis", "trend_forecasting"},
}
```

### Promises

Promises provide an asynchronous way to handle task results. When an agent is
assigned a task, it returns a Promise that will eventually resolve to the task
result.

Promises allow you to:

* Start a task and continue with other work while it's being processed
* Wait for a task to complete and retrieve its result
* Handle errors that might occur during task execution
* Coordinate multiple tasks with dependencies

Working with promises:

```go
// Start a task and get a promise
promise, err := agent.Work(ctx, task)
if err != nil {
    log.Fatal(err)
}

// Wait for the result (blocking)
result, err := promise.Get(ctx)
if err != nil {
    log.Fatal(err)
}

// Or wait for multiple promises
promises := []*dive.Promise{promise1, promise2, promise3}
results, err := dive.WaitAll(ctx, promises)
```

## LLM Integration

Dive provides a unified interface for working with different
Large Language Model (LLM) providers. This abstraction allows you to switch
between providers or models without changing your application code.

The framework currently supports:

* **Anthropic**
* **OpenAI**
* **Groq**

Each provider implementation handles the specifics of API communication, token
counting, error handling, and other provider-specific details, giving you a
consistent interface regardless of which provider you're using.

Key features of the LLM integration include:

* **Unified Interface**: Work with any supported LLM through the same API
* **Tool Use**: Enable LLMs to use tools to extend their capabilities
* **Caching**: Cache responses to reduce API costs and improve performance
* **Context Management**: Automatically handle context windows and token limits
* **Streaming Support**: Stream responses for better user experience **(in-progress)**

Using an LLM directly:

```go
// Create an LLM instance
llm := anthropic.New(anthropic.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")))

// Generate a response
response, err := llm.Generate(ctx, 
    []*llm.Message{llm.NewUserMessage("What is the capital of France?")},
    llm.WithSystemPrompt("You are a helpful assistant."),
)
```

### Supported Providers

Each provider has its own package but offers similar configuration options:

```go
// Anthropic
llm := anthropic.New(
    anthropic.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
    anthropic.WithModel("claude-3-7-sonnet-20250219"),
)

// OpenAI
llm := openai.New(
    openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    openai.WithModel("gpt-4o"),
)

// Groq
llm := groq.New(
    groq.WithAPIKey(os.Getenv("GROQ_API_KEY")),
    groq.WithModel("llama-3.3-70b-versatile"),
)
```

### Tested Models

These are the models that I have personally tested so far. Currently, Dive does
not restrict the model names you can specify, so feel free to try others.

| Provider  | Model                         |
| --------- | ----------------------------- |
| Anthropic | claude-3-5-sonnet-20240620    |
| Anthropic | claude-3-5-sonnet-20241022    |
| OpenAI    | gpt-4o                        |
| Groq      | llama-3.3-70b-versatile       |
| Groq      | deepseek-r1-distill-llama-70b |

### Tool Use

Tools extend the capabilities of agents by allowing them to perform actions
beyond just generating text. Tools can access external APIs, search the web,
retrieve data, perform calculations, and more.

Dive provides a simple interface for defining and using tools:

```go
type Tool interface {
	Definition() *ToolDefinition
	Call(ctx context.Context, input string) (string, error)
	ShouldReturnResult() bool
}
```

The framework includes a couple of built-in tools at this time:

* **Google Search**: Search the web for information
* **Firecrawl**: Extract content from web pages

Creating a custom tool is straightforward:

```go
type WeatherTool struct {
    apiKey string
}

func NewWeatherTool(apiKey string) *WeatherTool {
    return &WeatherTool{apiKey: apiKey}
}

func (t *WeatherTool) Definition() *llm.ToolDefinition {
    return &llm.ToolDefinition{
        Name:        "GetWeather",
        Description: "Get the current weather for a location",
        Parameters: llm.Schema{
            Type:     "object",
            Required: []string{"location"},
            Properties: map[string]*llm.SchemaProperty{
                "location": {
                    Type:        "string",
                    Description: "The city and state/country",
                },
            },
        },
    }
}

func (t *WeatherTool) Call(ctx context.Context, input string) (string, error) {
    // Parse input, call weather API, and return results
    // ...
}

func (t *WeatherTool) ShouldReturnResult() bool {
	return true
}
```

### Prompt Templates

Dive includes a flexible prompt templating system that helps you create
consistent, effective prompts for your agents. Templates can include variables
that are filled in at runtime, making it easy to create dynamic prompts.

Templates are defined using Go's text/template syntax and can include
conditional logic, loops, and other template features.

Example of using a prompt template:

```go
template := prompt.NewTemplate(`
You are a {{.Role}} assistant.
{{if .Context}}
Here is some context to help you:
{{.Context}}
{{end}}
Please help the user with their request.
`)

systemPrompt, err := template.Execute(map[string]interface{}{
    "Role":    "financial",
    "Context": "The user is asking about investment strategies.",
})
```

## Available Tools

Dive comes with several built-in tools that extend the capabilities of your
agents. These tools allow agents to interact with the outside world, access
information, and perform actions beyond just generating text.

### Google Search

The Google Search tool allows agents to search the web for information. This
uses the [Google Custom Search JSON API](https://developers.google.com/custom-search/v1/overview).

```go
// Create a Google Search API client
googleClient, err := google.New()
if err != nil {
    log.Fatal(err)
}

searchTool := tools.NewGoogleSearch(googleClient)

// Add it to an agent
agent := dive.NewAgent(dive.AgentOptions{
    // ... other configuration ...
    Tools: []dive.Tool{searchTool},
})
```

### Firecrawl

The [Firecrawl](https://www.firecrawl.dev) tool allows agents to extract content from web pages:

```go
// Create a Firecrawl tool
app, err := firecrawl.NewFirecrawlApp(os.Getenv("FIRECRAWL_API_KEY"), "")
scrapeTool := tools.NewFirecrawlScraper(app, 30000)

// Add it to an agent
agent := dive.NewAgent(dive.AgentOptions{
    // ... other configuration ...
    Tools: []dive.Tool{scrapeTool},
})
```

## Examples

Here are some examples of how to use Dive for common use cases:

### Single Agent Chat

Create a simple chat agent that can answer questions:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"

    "github.com/getstingrai/dive"
    "github.com/getstingrai/dive/llm"
    "github.com/getstingrai/dive/providers/anthropic"
)

func main() {
    ctx := context.Background()

    // Create an agent
    agent := dive.NewAgent(dive.AgentOptions{
        Name: "assistant",
        Role: &dive.Role{
            Description: "Helpful Assistant",
            AcceptsChats: true,
        },
        LLM:          anthropic.New(),
        CacheControl: "ephemeral",
    })

    // Start the agent
    if err := agent.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer agent.Stop(ctx)

    // Chat with the agent
    response, err := agent.Chat(ctx, llm.NewUserMessage("What is the capital of France?"))
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(response.Message().Text())
}
```

### Research Team

Create a team of agents to collaborate on research tasks:

```go
package main

import (
    "context"
    "fmt"
    "log"
    "os"
    "time"

    "github.com/getstingrai/dive"
    "github.com/getstingrai/dive/providers/anthropic"
    "github.com/getstingrai/dive/tools"
    "github.com/getstingrai/dive/tools/google"
)

func main() {
    ctx := context.Background()

    // Create Google Search tool
    googleClient := google.NewClient(
        os.Getenv("GOOGLE_SEARCH_API_KEY"),
        os.Getenv("GOOGLE_SEARCH_CX"),
    )
    searchTool := tools.NewGoogleSearch(googleClient)

    // Create agents
    supervisor := dive.NewAgent(dive.AgentOptions{
        Name: "supervisor",
        Role: &dive.Role{
            Description: "Research Team Lead",
            IsSupervisor: true,
            Subordinates: []string{"researcher"},
        },
        LLM:          anthropic.New(),
        CacheControl: "ephemeral",
    })

    researcher := dive.NewAgent(dive.AgentOptions{
        Name: "researcher",
        Role: &dive.Role{
            Description: "Research Assistant",
        },
        LLM:          anthropic.New(),
        CacheControl: "ephemeral",
        Tools:        []dive.Tool{searchTool},
    })

    // Create a team
    team, err := dive.NewTeam(dive.TeamOptions{
        Name:        "research-team",
        Description: "A team for conducting research",
        Agents:      []dive.Agent{supervisor, researcher},
    })
    if err != nil {
        log.Fatal(err)
    }

    // Start the team
    if err := team.Start(ctx); err != nil {
        log.Fatal(err)
    }
    defer team.Stop(ctx)

    // Create tasks
    researchTask := dive.NewTask(dive.TaskOptions{
        Name:        "research",
        Description: "Research the history of artificial intelligence",
        Timeout:     5 * time.Minute,
    })

    summaryTask := dive.NewTask(dive.TaskOptions{
        Name:         "summary",
        Description:  "Create a summary of the research findings",
        Dependencies: []string{"research"},
        Timeout:      5 * time.Minute,
    })

    // Assign tasks to the team
    results, err := team.Work(ctx, researchTask, summaryTask)
    if err != nil {
        log.Fatal(err)
    }

    // Print results
    for _, result := range results {
        fmt.Printf("Task: %s\nResult: %s\n\n", result.Task.Name(), result.Output)
    }
}
```

### Using YAML Configuration

Define and run a team using YAML configuration:

```yaml
# research_team.yaml
name: 'Research Team'
description: 'A team for conducting research'

config:
  default_provider: 'anthropic'
  log_level: 'info'
  cache_control: 'ephemeral'
  enabled_tools:
    - 'google_search'

agents:
  - name: 'Supervisor'
    role:
      description: 'Research Team Lead'
      is_supervisor: true
      subordinates:
        - 'Researcher'
  
  - name: 'Researcher'
    role:
      description: 'Research Assistant'
    tools:
      - 'google_search'

tasks:
  - name: 'Research'
    description: 'Research the history of artificial intelligence'
    assigned_agent: 'Researcher'
    timeout: '5m'

  - name: 'Summary'
    description: 'Create a summary of the research findings'
    assigned_agent: 'Supervisor'
    dependencies:
      - 'Research'
    timeout: '5m'
```

Run the YAML configuration:

```bash
go run cmd/yaml_runner/main.go -file=research_team.yaml -verbose
```

## Configuration

Dive provides flexible configuration options for agents, teams, and tasks. You
can configure these components either programmatically in Go or declaratively
using YAML or HCL.

### Agent Configuration

When creating an agent, you can configure:

* **Name**: A unique identifier for the agent
* **Role**: The agent's role, responsibilities, and capabilities
* **LLM Provider**: Which LLM provider to use (Anthropic, OpenAI, Groq)
* **Model**: Which specific model to use
* **Tools**: Which tools the agent can use
* **Cache Control**: How to handle caching of LLM responses
* **Task Limits**: Maximum number of concurrent tasks, timeouts, etc.

Example:

```go
agent := dive.NewAgent(dive.AgentOptions{
    Name: "researcher",
    Role: &dive.Role{
        Description: "Research Assistant",
        AcceptsWork: []string{"research"},
    },
    LLM:             anthropic.New(anthropic.WithModel("claude-3-7-sonnet-20250219")),
    Tools:           []dive.Tool{searchTool, scrapeTool},
    CacheControl:    "ephemeral",
    MaxActiveTasks:  5,
    TaskTimeout:     5 * time.Minute,
    ChatTimeout:     1 * time.Minute,
})
```

For YAML configuration, see the [examples/README.md](examples/README.md) file
for detailed information on the YAML structure and options.

## Contributing

We welcome contributions to Dive! Whether you're fixing bugs, adding features,
improving documentation, or spreading the word, your help is appreciated.

At this early stage, we're particularly interested in feedback on the concepts,
API design, usability, and any use cases you'd like to see supported.

## FAQ

### What makes Dive different from other agent frameworks?

Dive is designed with a focus on practical, production-ready agent systems.
Key differentiators include:

- First-class support for agent teams
- Simple concepts: agents, teams, tasks, roles
- Easy definition of tasks with dependencies
- Built-in support for multiple LLM providers
- Web search and scraping tools included
- Scalable to thousands of concurrent agents
- Great defaults so a lot works out-of-the-box

### How do I handle LLM rate limits?

Dive includes built-in retry mechanisms for handling rate limits. This includes
exponential backoff and jitter.

### Should I use Dive in production?

No, Dive is not recommended for production use at this time. As mentioned in the
Project Status section, Dive is in its early development stages and breaking
changes will occur as the API matures.

We recommend using it for experimentation, prototyping, and providing feedback
during this early stage. Once the project reaches a more stable state, we'll
provide clear guidance on production readiness.

### How can I extend Dive with custom functionality?

Dive is designed to be extensible:
- Create custom tools by implementing the `Tool` interface
- Implement custom agents by extending the base agent types
- Add support for new LLM providers by implementing the `LLM` interface
- Create custom prompt templates for specialized use cases

### What are the resource requirements?

Dive uses remote LLMs from the leading AI providers, so local resource usage is
minimal. Thousands of agents can run in parallel on a single machine.

### Is there a hosted or managed version available?

Not at this time. Dive is provided as an open-source framework that you can
self-host and integrate into your own applications.

## Who is Behind Dive?

Dive is developed by [Stingrai](https://www.getstingrai.com), a company building
products powered by agentic AI for competitive intelligence and product
messaging.
