package eval

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/risor-io/risor"
	"github.com/risor-io/risor/compiler"
	"github.com/risor-io/risor/object"
	"github.com/risor-io/risor/parser"
)

type String struct {
	raw   string
	codes []*compiler.Code
	parts []string
}

func NewString(raw string, globals map[string]any) (*String, error) {
	s := &String{
		raw: raw,
	}

	// First validate that all ${...} expressions are properly closed
	openCount := strings.Count(raw, "${")
	closeCount := strings.Count(raw, "}")
	if openCount > closeCount {
		return nil, fmt.Errorf("unclosed template expression in string: %q", raw)
	}

	// Compile all ${...} expressions into Risor code
	re := regexp.MustCompile(`\${([^}]+)}`)
	matches := re.FindAllStringSubmatchIndex(raw, -1)

	if len(matches) == 0 {
		// No template variables, just return the raw string
		s.parts = []string{raw}
		return s, nil
	}

	// Get sorted list of global names for compiler
	var globalNames []string
	for name := range globals {
		globalNames = append(globalNames, name)
	}
	sort.Strings(globalNames)

	var lastEnd int
	for _, match := range matches {
		// Add the text before the match
		if match[0] > lastEnd {
			s.parts = append(s.parts, raw[lastEnd:match[0]])
		}

		// Extract and compile the code inside ${...}
		expr := raw[match[2]:match[3]]
		ast, err := parser.Parse(context.Background(), expr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse template expression %q: %w", expr, err)
		}

		code, err := compiler.Compile(ast, compiler.WithGlobalNames(globalNames))
		if err != nil {
			return nil, fmt.Errorf("failed to compile template expression %q: %w", expr, err)
		}

		s.codes = append(s.codes, code)
		s.parts = append(s.parts, "") // Placeholder for the evaluated result
		lastEnd = match[1]
	}

	// Add any remaining text after the last match
	if lastEnd < len(raw) {
		s.parts = append(s.parts, raw[lastEnd:])
	}

	return s, nil
}

func (s *String) Eval(ctx context.Context, globals map[string]any) (string, error) {
	if len(s.codes) == 0 {
		// No template variables, return the raw string
		return s.raw, nil
	}

	// Make a copy of parts since we'll modify it
	parts := make([]string, len(s.parts))
	copy(parts, s.parts)

	// Evaluate each code block and replace the corresponding placeholder
	for _, code := range s.codes {
		result, err := risor.EvalCode(ctx, code, risor.WithGlobals(globals))
		if err != nil {
			return "", fmt.Errorf("failed to evaluate template expression: %w", err)
		}

		// Convert the result to a string based on its type
		var strValue string
		switch v := result.(type) {
		case *object.String:
			strValue = v.Value()
		case *object.Int:
			strValue = fmt.Sprintf("%d", v.Value())
		case *object.Float:
			strValue = fmt.Sprintf("%g", v.Value())
		case *object.Bool:
			strValue = fmt.Sprintf("%t", v.Value())
		case *object.NilType:
			strValue = ""
		default:
			return "", fmt.Errorf("unsupported result type for template expression: %T", result)
		}

		// Find the next empty placeholder and replace it
		for j := range parts {
			if parts[j] == "" {
				parts[j] = strValue
				break
			}
		}
	}

	// Join all parts to create the final string
	return strings.Join(parts, ""), nil
}
