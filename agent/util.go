package agent

import (
	"os"
	"regexp"

	"github.com/diveagents/dive/llm"
	"github.com/diveagents/dive/llm/providers/anthropic"
	"github.com/diveagents/dive/llm/providers/groq"
	"github.com/diveagents/dive/llm/providers/openai"
)

var newlinesRegex = regexp.MustCompile(`\n+`)

func replaceNewlines(text string) string {
	return newlinesRegex.ReplaceAllString(text, "<br>")
}

func detectProvider() (llm.LLM, bool) {
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		return anthropic.New(), true
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return openai.New(), true
	}
	if key := os.Getenv("GROQ_API_KEY"); key != "" {
		return groq.New(), true
	}
	return nil, false
}
