package llm

// NewMessage creates a new message with the given role and content blocks.
func NewMessage(role Role, content []Content) *Message {
	return &Message{Role: role, Content: content}
}

// NewUserMessage creates a new user message with the given content blocks.
func NewUserMessage(content ...Content) *Message {
	return &Message{Role: User, Content: content}
}

// NewUserTextMessage creates a new user message with a single text content
// block.
func NewUserTextMessage(text string) *Message {
	return &Message{
		Role:    User,
		Content: []Content{&TextContent{Text: text}},
	}
}

// NewAssistantMessage creates a new assistant message with the given content blocks.
func NewAssistantMessage(content ...Content) *Message {
	return &Message{Role: Assistant, Content: content}
}

// NewAssistantTextMessage creates a new assistant message with a single text
// content block.
func NewAssistantTextMessage(text string) *Message {
	return &Message{
		Role:    Assistant,
		Content: []Content{&TextContent{Text: text}},
	}
}

// NewToolResultMessage creates a new message with the user role and a list of
// tool outputs. Used to pass the results of tool calls back to an LLM.
func NewToolResultMessage(outputs ...*ToolResultContent) *Message {
	content := make([]Content, len(outputs))
	for i, output := range outputs {
		content[i] = &ToolResultContent{
			ToolUseID: output.ToolUseID,
			Content:   output.Content,
			// Name:      output.Name,
		}
	}
	return &Message{Role: User, Content: content}
}
