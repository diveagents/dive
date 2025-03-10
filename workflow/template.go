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
