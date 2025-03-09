package workflow

import (
	"bytes"
	"fmt"
	"html/template"
)

var (
	taskStatePromptTemplate *template.Template
	teamPromptTemplate      *template.Template
)

func init() {
	var err error
	taskStatePromptTemplate, err = parseTemplate("step_state_prompt", stepStatePromptText)
	if err != nil {
		panic(err)
	}
	teamPromptTemplate, err = parseTemplate("team_prompt", teamPromptText)
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

var teamPromptText = `{{- if .Description -}}
The team is described as: "{{ .Description }}"
{{- end }}

The team consists of the following agents:
{{ range $i, $agent := .Agents }}
- Name: {{ $agent.Name }}, Description: "{{ $agent.Description }}"
{{- end }}`

var stepStatePromptText = `# Step State

The step is described as: "{{ .Step.Description }}"

The step has the following dependencies:
{{- range .Step.Dependencies }}
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
