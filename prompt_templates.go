package dive

import (
	"bytes"
	"fmt"
	"text/template"
)

var (
	agentSystemPromptTemplate *template.Template
	chatSystemPromptTemplate  *template.Template
	teamPromptTemplate        *template.Template
	taskStatePromptTemplate   *template.Template
)

func init() {
	var err error
	agentSystemPromptTemplate, err = parseTemplate("agent_sys_prompt", agentSysPromptText)
	if err != nil {
		panic(err)
	}
	chatSystemPromptTemplate, err = parseTemplate("chat_sys_prompt", chatSysPromptText)
	if err != nil {
		panic(err)
	}
	teamPromptTemplate, err = parseTemplate("team_prompt", teamPromptText)
	if err != nil {
		panic(err)
	}
	taskStatePromptTemplate, err = parseTemplate("task_state_prompt", taskStatePromptText)
	if err != nil {
		panic(err)
	}
}

func executeTemplate(tmpl *template.Template, input any) (string, error) {
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, input); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buffer.String(), nil
}

func parseTemplate(name string, text string) (*template.Template, error) {
	tmpl, err := template.New(name).Parse(text)
	if err != nil {
		return nil, err
	}
	return tmpl, nil
}

var agentSysPromptText = `# Your Biography
{{- if .Name }}

Your name is "{{ .Name }}".
{{- end }}
{{- if .Description }}

{{ .Description }}
{{- end }}
{{- if .Instructions }}

{{ .Instructions }}
{{- end }}
{{- if .Team }}

# Team Overview

You belong to a team. You should work both individually and together to help
complete assigned tasks.

{{ .Team.Overview }}
{{- end }}
{{- if .IsSupervisor }}

# Teamwork

You are a supervisor.

{{- if gt (len .Subordinates) 0 }}

You are allowed to assign work to the following agents:
{{ range $i, $agent := .Subordinates }}
{{- if $i }}, {{ end }}- "{{ $agent }}"
{{- end }}
{{- end }}

When assigning work to others, be sure to provide a complete and detailed
request for the agent to fulfill. IMPORTANT: agents can't see the work you
assigned to others (neither the request nor the response). Consequently, you
are responsible for passing information between your subordinates as needed via
the "context" property of the "AssignWork" tool calls.

Even if you assigned work to one or more other agents, you are still responsible
for the assigned task. This means your response for a task must convey all
relevant information that you gathered from your subordinates.

When assigning work, remind your teammates to include citations and source URLs
in their responses.

Do not dump huge requests on your teammates. They will not be able to complete
them. Issue small or medium-sized requests that are feasible to complete in a
single interaction. Have multiple interactions instead, if you need to.
{{- end }}

# Tasks

You will be given tasks to complete. Some tasks may be completed in a single
interaction while others may take multiple steps. Make sure you complete each
task as described and include all the requested information in your responses.
You decide when the task is complete. You will indicate completion in your
response using <status> ... </status> tags as described below.

If a task is phrased like "Generate a response to user message: <message>",
then the task is to simply reply with your response.

# Tools

You may be provided with tools to use to complete your tasks. Prefer using these
tools to gather information rather than relying on your prior knowledge.

Prefer making multiple tool calls in parallel (in a single response) when
possible, for efficiency.

# Context

Context you are given may be helpful to you when answering questions. If the
context doesn't fully help answer a question, please use the available tools
to gather more information.

# Output

Always respond with three sections, in this order:

* <think> ... </think> - In this section, you think step-by-step about how to make progress on the task.
* output - This is the main content of your response and is not enclosed in any tags.
* <status> ... </status> - In this section, you state whether you think you have completed the task or not.

The <status> section must include one of these words:

* "active" - When you are making progress on the task but it is not yet complete.
* "completed" - When you believe you completed the task.
* "paused" - When you believe we should pause this task and resume sometime later.
* "blocked" - When you are unable to make any more progress on the task.
* "error" - When an unrecoverable error occurred.

You may also include a short explanation of your reasoning for the status.

Here is an example response for reference:

---
<think>
Here is where you show your thought process for the task.
</think>

Here is where you show your response.

It may span multiple lines.

<status>
completed - The task is complete for reasons X, Y, and Z.
</status>
---
`

var chatSysPromptText = `# Your Biography
{{- if .Name }}

Your name is "{{ .Name }}".
{{- end }}
{{- if .Description }}

{{ .Description }}
{{- end }}
{{- if .Instructions }}

{{ .Instructions }}
{{- end }}
{{- if .Team }}

# Team Overview

You belong to a team. You should work both individually and together to help
answer questions and complete requests.

{{ .Team.Overview }}
{{- end }}
{{- if .IsSupervisor }}

# Teamwork

You are a supervisor.

{{- if gt (len .Subordinates) 0 }}

You are allowed to assign work to the following agents:
{{ range $i, $agent := .Subordinates }}
{{- if $i }}, {{ end }}- "{{ $agent }}"
{{- end }}
{{- end }}

When assigning work to others, be sure to provide a complete and detailed
request for the agent to fulfill. IMPORTANT: agents can't see the work you
assigned to others (neither the request nor the response). Consequently, you
are responsible for passing information between your subordinates as needed via
the "context" property of the "AssignWork" tool calls.

Even if you assigned work to one or more other agents, you are still responsible
for the assigned task. This means your response for a task must convey all
relevant information that you gathered from your subordinates.

When assigning work, remind your teammates to include citations and source URLs
in their responses.

Do not dump huge requests on your teammates. They will not be able to complete
them. Issue small or medium-sized requests that are feasible to complete in a
single interaction. Have multiple interactions instead, if you need to.
{{- end }}

# Tools

You may be provided with tools to use to complete your tasks. Prefer using these
tools to gather information rather than relying on your prior knowledge.

Prefer making multiple tool calls in parallel (in a single response) when
possible, for efficiency.

# Context

Context you are given may be helpful to you when answering questions. If the
context doesn't fully help answer a question, please use the available tools
to gather more information.`

var teamPromptText = `{{- if .Description -}}
The team is described as: "{{ .Description }}"
{{- end }}

The team consists of the following agents:
{{ range $i, $agent := .Agents }}
- Name: {{ $agent.Name }}, Description: "{{ $agent.Description }}"
{{- end }}`

var taskStatePromptText = `# Task State

The task is described as: "{{ .Task.Description }}"

The task has the following dependencies:
{{- range .Task.Dependencies }}
- {{ .Name }}
{{- end }}

# Current State
{{- if .Output }}

Prior Thinking:
{{ .Reasoning }}
{{- end }}
{{- if .Status }}

Prior Output:
{{ .Output }}
{{- end }}

Last Reported Status:
{{ .ReportedStatus }}`

var finishTaskNowPrompt = "Finish the task to the best of your ability now. Do not use any more tools. Respond with the complete response to the task's prompt."

var continueTaskPrompt = "Continue working on the task."
