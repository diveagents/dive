package workflow

import (
	"bytes"
	"fmt"
	"html/template"
)

func executeTemplate(tmpl *template.Template, input any) (string, error) {
	var buffer bytes.Buffer
	if err := tmpl.Execute(&buffer, input); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buffer.String(), nil
}
