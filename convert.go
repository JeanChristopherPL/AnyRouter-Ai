package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type CanonicalMessage struct {
	Role       string              `json:"role"`
	Content    interface{}         `json:"content"`
	ToolCalls  []ToolCall          `json:"tool_calls,omitempty"`
	ToolCallID string              `json:"tool_call_id,omitempty"`
	Name       string              `json:"name,omitempty"`
}

type ToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function ToolCallFunction `json:"function"`
}

type ToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type CanonicalTool struct {
	Type     string          `json:"type"`
	Function json.RawMessage `json:"function,omitempty"`
}

type AnthropicContentBlock struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	ID    string `json:"id,omitempty"`
	Name  string `json:"name,omitempty"`
	Input any    `json:"input,omitempty"`
}

type AnthropicRequest struct {
	Model         string                   `json:"model"`
	Messages      []AnthropicRequestMessage `json:"messages"`
	System        string                   `json:"system,omitempty"`
	MaxTokens     int                      `json:"max_tokens"`
	Temperature   *float64                 `json:"temperature,omitempty"`
	Stream        bool                     `json:"stream"`
	Tools         []AnthropicTool          `json:"tools,omitempty"`
	ToolChoice    any                      `json:"tool_choice,omitempty"`
	StopSequences []string                 `json:"stop_sequences,omitempty"`
	TopP          *float64                 `json:"top_p,omitempty"`
}

type AnthropicRequestMessage struct {
	Role    string                   `json:"role"`
	Content []AnthropicContentBlock  `json:"content"`
}

type AnthropicTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	InputSchema any    `json:"input_schema"`
}

type CanonicalChatRequest struct {
	Model       string              `json:"model"`
	Messages    []CanonicalMessage  `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
	Tools       []CanonicalTool     `json:"tools,omitempty"`
	ToolChoice  any                 `json:"tool_choice,omitempty"`
	Stop        []string            `json:"stop,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
}

type CanonicalChatResponse struct {
	ID      string            `json:"id"`
	Object  string            `json:"object"`
	Created int64             `json:"created"`
	Model   string            `json:"model"`
	Choices []CanonicalChoice `json:"choices"`
	Usage   *Usage            `json:"usage,omitempty"`
}

type CanonicalChoice struct {
	Index        int              `json:"index"`
	Message      CanonicalMessage `json:"message"`
	FinishReason string           `json:"finish_reason"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type OpenAIChatRequest struct {
	Model       string              `json:"model"`
	Messages    []OpenAIMessage     `json:"messages"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream"`
	Tools       []CanonicalTool     `json:"tools,omitempty"`
	ToolChoice  any                 `json:"tool_choice,omitempty"`
	Stop        []string            `json:"stop,omitempty"`
	TopP        float64             `json:"top_p,omitempty"`
}

type OpenAIMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

type OpenAIResponse struct {
	ID      string          `json:"id"`
	Object  string          `json:"object"`
	Created int64           `json:"created"`
	Model   string          `json:"model"`
	Choices []OpenAIChoice  `json:"choices"`
	Usage   *Usage          `json:"usage,omitempty"`
}

type OpenAIChoice struct {
	Index        int           `json:"index"`
	Message      OpenAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type AnthropicResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []AnthropicContentBlock  `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence any                      `json:"stop_sequence"`
	Usage        *AnthropicUsage          `json:"usage,omitempty"`
}

type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type GeminiRequest struct {
	Contents        []GeminiContent    `json:"contents"`
	SystemInstruction *GeminiContent   `json:"system_instruction,omitempty"`
	GenerationConfig GeminiGenConfig     `json:"generation_config,omitempty"`
	Tools           []GeminiTool        `json:"tools,omitempty"`
}

type GeminiContent struct {
	Role  string       `json:"role"`
	Parts []GeminiPart `json:"parts"`
}

type GeminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *GeminiInlineData `json:"inline_data,omitempty"`
}

type GeminiInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type GeminiGenConfig struct {
	Temperature      float64  `json:"temperature,omitempty"`
	MaxOutputTokens  int      `json:"max_output_tokens,omitempty"`
	TopP             float64  `json:"top_p,omitempty"`
	StopSequences    []string `json:"stop_sequences,omitempty"`
}

type GeminiTool struct {
	FunctionDeclarations []GeminiFunctionDecl `json:"function_declarations"`
}

type GeminiFunctionDecl struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type GeminiResponse struct {
	Candidates []GeminiCandidate `json:"candidates"`
	Usage      *GeminiUsageMeta  `json:"usageMetadata,omitempty"`
}

type GeminiCandidate struct {
	Content      GeminiContent `json:"content"`
	FinishReason string        `json:"finishReason"`
}

type GeminiUsageMeta struct {
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

type CohereRequest struct {
	Message     string             `json:"message"`
	Model       string             `json:"model"`
	ChatHistory []CohereChatMessage `json:"chat_history,omitempty"`
	Stream      bool               `json:"stream"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature float64            `json:"temperature,omitempty"`
	Tools       []CohereTool       `json:"tools,omitempty"`
}

type CohereChatMessage struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

type CohereTool struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// ─── Parse Input Formats ──────────────────────────────────────────────

func ParseOpenAIRequest(body []byte) (*CanonicalChatRequest, error) {
	var oai OpenAIChatRequest
	if err := json.Unmarshal(body, &oai); err != nil {
		return nil, fmt.Errorf("parse openai request: %w", err)
	}
	canonical := &CanonicalChatRequest{
		Model:       oai.Model,
		MaxTokens:   oai.MaxTokens,
		Temperature: oai.Temperature,
		Stream:      oai.Stream,
		Tools:       oai.Tools,
		ToolChoice:  oai.ToolChoice,
		Stop:        oai.Stop,
		TopP:        oai.TopP,
	}
	for _, m := range oai.Messages {
		cm := CanonicalMessage{Role: m.Role, Content: m.Content, ToolCalls: m.ToolCalls, ToolCallID: m.ToolCallID, Name: m.Name}
		canonical.Messages = append(canonical.Messages, cm)
	}
	return canonical, nil
}

func ParseAnthropicRequest(body []byte) (*CanonicalChatRequest, error) {
	var ar AnthropicRequest
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}
	canonical := &CanonicalChatRequest{
		Model:     ar.Model,
		MaxTokens: ar.MaxTokens,
		Stream:    ar.Stream,
		Tools:     nil,
		Stop:      ar.StopSequences,
	}
	if ar.Temperature != nil {
		canonical.Temperature = *ar.Temperature
	}
	if ar.TopP != nil {
		canonical.TopP = *ar.TopP
	}
	if ar.System != "" {
		canonical.Messages = append(canonical.Messages, CanonicalMessage{Role: "system", Content: ar.System})
	}
	for _, m := range ar.Messages {
		role := m.Role
		if role == "assistant" {
			var textParts []string
			var toolCalls []ToolCall
			for _, block := range m.Content {
				switch block.Type {
				case "text":
					textParts = append(textParts, block.Text)
				case "tool_use":
					inputJSON, _ := json.Marshal(block.Input)
					tc := ToolCall{
						ID:   block.ID,
						Type: "function",
						Function: ToolCallFunction{
							Name:      block.Name,
							Arguments: string(inputJSON),
						},
					}
					toolCalls = append(toolCalls, tc)
				}
			}
			content := strings.Join(textParts, "")
			if content == "" && len(toolCalls) == 0 {
				content = ""
			}
			cm := CanonicalMessage{Role: role, Content: content, ToolCalls: toolCalls}
			canonical.Messages = append(canonical.Messages, cm)
		} else {
			var textParts []string
			for _, block := range m.Content {
				switch block.Type {
				case "text":
					textParts = append(textParts, block.Text)
				case "tool_result":
					cm := CanonicalMessage{Role: "tool", Content: block.Text, ToolCallID: block.ID}
					canonical.Messages = append(canonical.Messages, cm)
				case "image":
					textParts = append(textParts, "[image]")
				}
			}
			if len(textParts) > 0 {
				cm := CanonicalMessage{Role: role, Content: strings.Join(textParts, "")}
				canonical.Messages = append(canonical.Messages, cm)
			}
		}
	}
	return canonical, nil
}

func ParseGeminiRequest(body []byte) (*CanonicalChatRequest, error) {
	var gr GeminiRequest
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("parse gemini request: %w", err)
	}
	canonical := &CanonicalChatRequest{
		MaxTokens:   gr.GenerationConfig.MaxOutputTokens,
		Temperature: gr.GenerationConfig.Temperature,
		TopP:        gr.GenerationConfig.TopP,
		Stop:        gr.GenerationConfig.StopSequences,
	}
	if gr.SystemInstruction != nil {
		text := ""
		for _, p := range gr.SystemInstruction.Parts {
			text += p.Text
		}
		canonical.Messages = append(canonical.Messages, CanonicalMessage{Role: "system", Content: text})
	}
	roleMap := map[string]string{"user": "user", "model": "assistant"}
	for _, c := range gr.Contents {
		role := roleMap[c.Role]
		if role == "" {
			role = "user"
		}
		var textParts []string
		for _, p := range c.Parts {
			textParts = append(textParts, p.Text)
		}
		canonical.Messages = append(canonical.Messages, CanonicalMessage{Role: role, Content: strings.Join(textParts, "")})
	}
	return canonical, nil
}

func ParseToCanonical(format string, body []byte) (*CanonicalChatRequest, error) {
	switch format {
	case "anthropic":
		return ParseAnthropicRequest(body)
	case "gemini":
		return ParseGeminiRequest(body)
	default:
		return ParseOpenAIRequest(body)
	}
}

// ─── Convert Canonical → Native ──────────────────────────────────────

func ConvertCanonicalToNative(format string, req *CanonicalChatRequest) ([]byte, error) {
	switch format {
	case "anthropic":
		return convertToAnthropic(req)
	case "gemini":
		return convertToGemini(req)
	case "cohere":
		return convertToCohere(req)
	default:
		return convertToOpenAI(req)
	}
}

func convertToOpenAI(req *CanonicalChatRequest) ([]byte, error) {
	oai := OpenAIChatRequest{
		Model:       req.Model,
		MaxTokens:   req.MaxTokens,
		Temperature: req.Temperature,
		Stream:      req.Stream,
		Tools:       req.Tools,
		ToolChoice:  req.ToolChoice,
		Stop:        req.Stop,
		TopP:        req.TopP,
	}
	for _, m := range req.Messages {
		oai.Messages = append(oai.Messages, OpenAIMessage{
			Role:       m.Role,
			Content:    m.Content,
			ToolCalls:  m.ToolCalls,
			ToolCallID: m.ToolCallID,
			Name:       m.Name,
		})
	}
	return json.Marshal(oai)
}

func convertToAnthropic(req *CanonicalChatRequest) ([]byte, error) {
	ar := AnthropicRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		Stream:    req.Stream,
		Tools:     nil,
	}
	if req.MaxTokens == 0 {
		ar.MaxTokens = 4096
	}
	if req.Temperature != 0 {
		ar.Temperature = &req.Temperature
	}
	if req.TopP != 0 {
		ar.TopP = &req.TopP
	}
	if len(req.Stop) > 0 {
		ar.StopSequences = req.Stop
	}

	// Handle tools
	for _, t := range req.Tools {
		at := AnthropicTool{Name: "unknown", InputSchema: map[string]any{"type": "object", "properties": map[string]any{}}}
		if len(t.Function) > 0 {
			var fn struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Parameters  any    `json:"parameters"`
			}
			if err := json.Unmarshal(t.Function, &fn); err == nil {
				at.Name = fn.Name
				at.Description = fn.Description
				at.InputSchema = fn.Parameters
			}
		}
		ar.Tools = append(ar.Tools, at)
	}

	// Handle tool_choice mapping
	if req.ToolChoice != nil {
		switch tc := req.ToolChoice.(type) {
		case string:
			ar.ToolChoice = tc
		case map[string]any:
			if t, ok := tc["type"].(string); ok && t == "function" {
				if fn, ok := tc["function"].(map[string]any); ok {
					if name, ok := fn["name"].(string); ok {
						ar.ToolChoice = map[string]any{"type": "tool", "name": name}
					}
				}
			} else {
				ar.ToolChoice = tc
			}
		}
	}

	// Separate system message
	var systemParts []string
	var msgs []CanonicalMessage
	for _, m := range req.Messages {
		if m.Role == "system" {
			if s, ok := m.Content.(string); ok {
				systemParts = append(systemParts, s)
			}
		} else {
			msgs = append(msgs, m)
		}
	}
	if len(systemParts) > 0 {
		ar.System = strings.Join(systemParts, "\n")
	}

	// Convert messages
	for _, m := range msgs {
		role := m.Role
		if role == "tool" {
			role = "user"
		}
		arm := AnthropicRequestMessage{Role: role}

		switch m.Role {
		case "assistant":
			if len(m.ToolCalls) > 0 {
				for _, tc := range m.ToolCalls {
					if tc.Type == "function" {
						arm.Content = append(arm.Content, AnthropicContentBlock{
							Type:  "tool_use",
							ID:    tc.ID,
							Name:  tc.Function.Name,
							Input: json.RawMessage(tc.Function.Arguments),
						})
					}
				}
			}
			if m.Content != nil {
				contentStr := contentToString(m.Content)
				if contentStr != "" {
					arm.Content = append([]AnthropicContentBlock{{Type: "text", Text: contentStr}}, arm.Content...)
				}
			}
		case "tool":
			arm.Content = []AnthropicContentBlock{{
				Type: "tool_result",
				ID:   m.ToolCallID,
				Text: contentToString(m.Content),
			}}
		default:
			arm.Content = []AnthropicContentBlock{{Type: "text", Text: contentToString(m.Content)}}
		}
		ar.Messages = append(ar.Messages, arm)
	}

	return json.Marshal(ar)
}

func convertToGemini(req *CanonicalChatRequest) ([]byte, error) {
	gr := GeminiRequest{
		GenerationConfig: GeminiGenConfig{
			Temperature:     req.Temperature,
			MaxOutputTokens: req.MaxTokens,
			TopP:            req.TopP,
			StopSequences:   req.Stop,
		},
	}

	for _, m := range req.Messages {
		if m.Role == "system" {
			if s, ok := m.Content.(string); ok && s != "" {
				gr.SystemInstruction = &GeminiContent{
					Parts: []GeminiPart{{Text: s}},
				}
			}
			continue
		}
		gemRole := "user"
		if m.Role == "assistant" {
			gemRole = "model"
		}
		parts := []GeminiPart{{Text: contentToString(m.Content)}}
		gr.Contents = append(gr.Contents, GeminiContent{Role: gemRole, Parts: parts})
	}

	// Convert tools
	if len(req.Tools) > 0 {
		for _, t := range req.Tools {
			if len(t.Function) > 0 {
				var fd GeminiFunctionDecl
				if err := json.Unmarshal(t.Function, &fd); err == nil {
					gr.Tools = append(gr.Tools, GeminiTool{
						FunctionDeclarations: []GeminiFunctionDecl{fd},
					})
				}
			}
		}
	}

	return json.Marshal(gr)
}

func convertToCohere(req *CanonicalChatRequest) ([]byte, error) {
	cr := CohereRequest{
		Model:       req.Model,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
	}

	for i, m := range req.Messages {
		switch m.Role {
		case "system":
			if i == len(req.Messages)-1 {
				cr.Message = contentToString(m.Content)
			} else {
				cr.ChatHistory = append(cr.ChatHistory, CohereChatMessage{Role: "SYSTEM", Message: contentToString(m.Content)})
			}
		case "user":
			if i == len(req.Messages)-1 {
				cr.Message = contentToString(m.Content)
			} else {
				cr.ChatHistory = append(cr.ChatHistory, CohereChatMessage{Role: "USER", Message: contentToString(m.Content)})
			}
		case "assistant":
			cr.ChatHistory = append(cr.ChatHistory, CohereChatMessage{Role: "CHATBOT", Message: contentToString(m.Content)})
		}
	}

	return json.Marshal(cr)
}

// ─── Convert Native Response → Canonical ─────────────────────────────

type CanonicalNativeResponse struct {
	Body        []byte
	ContentType string
}

func ConvertNativeToCanonical(format string, resp *http.Response) (*CanonicalChatResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response body: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("%s API error (%d): %s", format, resp.StatusCode, string(body))
	}

	switch format {
	case "anthropic":
		return parseAnthropicResponse(body)
	case "gemini":
		return parseGeminiResponse(body)
	default:
		return parseOpenAIResponse(body)
	}
}

func parseOpenAIResponse(body []byte) (*CanonicalChatResponse, error) {
	var oaiResp OpenAIResponse
	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return nil, fmt.Errorf("parse openai response: %w", err)
	}
	ccr := &CanonicalChatResponse{
		ID:      oaiResp.ID,
		Object:  oaiResp.Object,
		Created: oaiResp.Created,
		Model:   oaiResp.Model,
		Usage:   oaiResp.Usage,
	}
	for _, c := range oaiResp.Choices {
		ccr.Choices = append(ccr.Choices, CanonicalChoice{
			Index: c.Index,
			Message: CanonicalMessage{
				Role:      c.Message.Role,
				Content:   c.Message.Content,
				ToolCalls: c.Message.ToolCalls,
			},
			FinishReason: c.FinishReason,
		})
	}
	return ccr, nil
}

func parseAnthropicResponse(body []byte) (*CanonicalChatResponse, error) {
	var ar AnthropicResponse
	if err := json.Unmarshal(body, &ar); err != nil {
		return nil, fmt.Errorf("parse anthropic response: %w", err)
	}

	ccr := &CanonicalChatResponse{
		ID:     ar.ID,
		Object: "chat.completion",
		Model:  ar.Model,
		Usage: &Usage{
			PromptTokens:     ar.Usage.InputTokens,
			CompletionTokens: ar.Usage.OutputTokens,
			TotalTokens:      ar.Usage.InputTokens + ar.Usage.OutputTokens,
		},
	}

	finishReason := ar.StopReason
	switch finishReason {
	case "end_turn", "stop_sequence":
		finishReason = "stop"
	case "tool_use":
		finishReason = "tool_calls"
	case "max_tokens":
		finishReason = "length"
	}

	var textParts []string
	var toolCalls []ToolCall
	for _, block := range ar.Content {
		switch block.Type {
		case "text":
			textParts = append(textParts, block.Text)
		case "tool_use":
			inputJSON, _ := json.Marshal(block.Input)
			tc := ToolCall{
				ID:   block.ID,
				Type: "function",
				Function: ToolCallFunction{
					Name:      block.Name,
					Arguments: string(inputJSON),
				},
			}
			toolCalls = append(toolCalls, tc)
		}
	}

	content := strings.Join(textParts, "")
	msg := CanonicalMessage{
		Role:      "assistant",
		Content:   content,
		ToolCalls: toolCalls,
	}

	ccr.Choices = []CanonicalChoice{{
		Index:        0,
		Message:      msg,
		FinishReason: finishReason,
	}}

	return ccr, nil
}

func parseGeminiResponse(body []byte) (*CanonicalChatResponse, error) {
	var gr GeminiResponse
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("parse gemini response: %w", err)
	}

	ccr := &CanonicalChatResponse{
		Object: "chat.completion",
	}
	if gr.Usage != nil {
		ccr.Usage = &Usage{
			PromptTokens:     gr.Usage.PromptTokenCount,
			CompletionTokens: gr.Usage.CandidatesTokenCount,
			TotalTokens:      gr.Usage.TotalTokenCount,
		}
	}

	for i, c := range gr.Candidates {
		var textParts []string
		for _, p := range c.Content.Parts {
			textParts = append(textParts, p.Text)
		}
		fr := strings.ToLower(c.FinishReason)
		switch fr {
		case "stop":
			fr = "stop"
		case "max_tokens":
			fr = "length"
		case "safety":
			fr = "content_filter"
		default:
			fr = "stop"
		}
		ccr.Choices = append(ccr.Choices, CanonicalChoice{
			Index: i,
			Message: CanonicalMessage{
				Role:    "assistant",
				Content: strings.Join(textParts, ""),
			},
			FinishReason: fr,
		})
	}

	return ccr, nil
}

// ─── Convert Canonical Response → Output Format ──────────────────────

func ConvertCanonicalToOutput(outputFormat string, ccr *CanonicalChatResponse) ([]byte, error) {
	switch outputFormat {
	case "anthropic":
		return convertCanonicalToAnthropicOutput(ccr)
	case "gemini":
		return convertCanonicalToGeminiOutput(ccr)
	default:
		return convertCanonicalToOpenAIOutput(ccr)
	}
}

func convertCanonicalToOpenAIOutput(ccr *CanonicalChatResponse) ([]byte, error) {
	oaiResp := OpenAIResponse{
		ID:      ccr.ID,
		Object:  ccr.Object,
		Created: ccr.Created,
		Model:   ccr.Model,
		Usage:   ccr.Usage,
	}
	for _, c := range ccr.Choices {
		oaiResp.Choices = append(oaiResp.Choices, OpenAIChoice{
			Index: c.Index,
			Message: OpenAIMessage{
				Role:      c.Message.Role,
				Content:   c.Message.Content,
				ToolCalls: c.Message.ToolCalls,
			},
			FinishReason: c.FinishReason,
		})
	}
	return json.Marshal(oaiResp)
}

func convertCanonicalToAnthropicOutput(ccr *CanonicalChatResponse) ([]byte, error) {
	ar := AnthropicResponse{
		ID:    ccr.ID,
		Type:  "message",
		Role:  "assistant",
		Model: ccr.Model,
	}

	if ccr.Usage != nil {
		ar.Usage = &AnthropicUsage{
			InputTokens:  ccr.Usage.PromptTokens,
			OutputTokens: ccr.Usage.CompletionTokens,
		}
	}

	for _, c := range ccr.Choices {
		finishReason := c.FinishReason
		switch finishReason {
		case "stop":
			finishReason = "end_turn"
		case "tool_calls":
			finishReason = "tool_use"
		case "length":
			finishReason = "max_tokens"
		}

		if ar.StopReason == "" {
			ar.StopReason = finishReason
		}

		msg := c.Message
		if msg.Content != nil {
			contentStr := contentToString(msg.Content)
			if contentStr != "" {
				ar.Content = append(ar.Content, AnthropicContentBlock{Type: "text", Text: contentStr})
			}
		}
		for _, tc := range msg.ToolCalls {
			if tc.Type == "function" {
				var input any
				json.Unmarshal([]byte(tc.Function.Arguments), &input)
				ar.Content = append(ar.Content, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
		}
	}

	if ar.Content == nil {
		ar.Content = []AnthropicContentBlock{}
	}

	return json.Marshal(ar)
}

func convertCanonicalToGeminiOutput(ccr *CanonicalChatResponse) ([]byte, error) {
	gr := GeminiResponse{}
	for _, c := range ccr.Choices {
		fr := strings.ToUpper(c.FinishReason)
		if fr == "TOOL_CALLS" {
			fr = "STOP"
		}
		candidate := GeminiCandidate{
			Content: GeminiContent{
				Role:  "model",
				Parts: []GeminiPart{{Text: contentToString(c.Message.Content)}},
			},
			FinishReason: fr,
		}
		gr.Candidates = append(gr.Candidates, candidate)
	}
	if ccr.Usage != nil {
		gr.Usage = &GeminiUsageMeta{
			PromptTokenCount:     ccr.Usage.PromptTokens,
			CandidatesTokenCount: ccr.Usage.CompletionTokens,
			TotalTokenCount:      ccr.Usage.TotalTokens,
		}
	}
	return json.Marshal(gr)
}

// ─── Streaming ────────────────────────────────────────────────────────

func ConvertStreamChunk(format string, chunk []byte) ([]byte, error) {
	// First convert provider stream chunk to canonical delta
	canonicalDelta, err := parseStreamChunkToDelta(format, chunk)
	if err != nil {
		return nil, err
	}

	// Default output is OpenAI format
	return marshalOpenAIStreamDelta(canonicalDelta)
}

func parseStreamChunkToDelta(format string, chunk []byte) (*StreamDelta, error) {
	chunk = bytes.TrimSpace(chunk)
	if len(chunk) == 0 {
		return nil, nil
	}

	switch format {
	case "anthropic":
		return parseAnthropicStreamChunk(chunk)
	case "gemini":
		return parseGeminiStreamChunk(chunk)
	default:
		return parseOpenAIStreamChunk(chunk)
	}
}

type StreamDelta struct {
	Role         string      `json:"role,omitempty"`
	Content      string      `json:"content,omitempty"`
	ToolCalls    []ToolCall  `json:"tool_calls,omitempty"`
	FinishReason string      `json:"finish_reason,omitempty"`
	Usage        *Usage      `json:"usage,omitempty"`
}

type OpenAIStreamChunk struct {
	ID      string                `json:"id"`
	Object  string                `json:"object"`
	Created int64                 `json:"created"`
	Model   string                `json:"model"`
	Choices []OpenAIStreamChoice  `json:"choices"`
}

type OpenAIStreamChoice struct {
	Index int              `json:"index"`
	Delta OpenAIMessage    `json:"delta"`
	FinishReason *string   `json:"finish_reason"`
}

func parseOpenAIStreamChunk(chunk []byte) (*StreamDelta, error) {
	var osc OpenAIStreamChunk
	if err := json.Unmarshal(chunk, &osc); err != nil {
		return nil, nil // skip unparseable chunks (like "[DONE]")
	}
	sd := &StreamDelta{}
	for _, c := range osc.Choices {
		sd.Content += contentToString(c.Delta.Content)
		if c.FinishReason != nil && *c.FinishReason != "" {
			sd.FinishReason = *c.FinishReason
		}
	}
	return sd, nil
}

type AnthropicStreamEvent struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

type AnthropicContentBlockDelta struct {
	Type  string `json:"type"`
	Index int    `json:"index"`
	Delta *struct {
		Text        string `json:"text,omitempty"`
		PartialJSON string `json:"partial_json,omitempty"`
	} `json:"delta,omitempty"`
}

type AnthropicContentBlockStart struct {
	Type        string                `json:"type"`
	Index       int                   `json:"index"`
	ContentBlock *AnthropicContentBlock `json:"content_block,omitempty"`
}

type AnthropicMessageStart struct {
	Type  string           `json:"type"`
	Message *AnthropicResponse `json:"message,omitempty"`
}

func parseAnthropicStreamChunk(chunk []byte) (*StreamDelta, error) {
	// Anthropic SSE uses lines like:
	// event: content_block_delta\ndata: {"type":"content_block_delta","delta":{"text":"..."}}
	// We may receive just the data JSON or the full event
	var event AnthropicStreamEvent
	if err := json.Unmarshal(chunk, &event); err != nil {
		// Maybe it's a bare data JSON
		event.Data = chunk
	}

	data := event.Data
	if len(data) == 0 {
		data = chunk
	}

	// Try to determine the event type from the data
	var typeHolder struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(data, &typeHolder); err != nil {
		return nil, nil
	}

	sd := &StreamDelta{}

	switch typeHolder.Type {
	case "content_block_delta":
		var delta AnthropicContentBlockDelta
		if err := json.Unmarshal(data, &delta); err == nil && delta.Delta != nil {
			sd.Content = delta.Delta.Text
		}
	case "content_block_start":
		var start AnthropicContentBlockStart
		if err := json.Unmarshal(data, &start); err == nil && start.ContentBlock != nil {
			sd.Content = start.ContentBlock.Text
		}
	case "message_start":
		var ms AnthropicMessageStart
		if err := json.Unmarshal(data, &ms); err == nil && ms.Message != nil {
			sd.Role = "assistant"
		}
	case "message_delta":
		var md struct {
			Type  string `json:"type"`
			Delta *struct {
				StopReason string `json:"stop_reason"`
			} `json:"delta"`
			Usage *AnthropicUsage `json:"usage"`
		}
		if err := json.Unmarshal(data, &md); err == nil {
			if md.Delta != nil {
				switch md.Delta.StopReason {
				case "end_turn", "stop_sequence":
					sd.FinishReason = "stop"
				case "tool_use":
					sd.FinishReason = "tool_calls"
				case "max_tokens":
					sd.FinishReason = "length"
				}
			}
			if md.Usage != nil {
				sd.Usage = &Usage{
					PromptTokens:     md.Usage.InputTokens,
					CompletionTokens: md.Usage.OutputTokens,
					TotalTokens:      md.Usage.InputTokens + md.Usage.OutputTokens,
				}
			}
		}
	case "message_stop":
		sd.FinishReason = "stop"
	}

	return sd, nil
}

func parseGeminiStreamChunk(chunk []byte) (*StreamDelta, error) {
	// Gemini streaming returns JSON objects similar to non-streaming
	var gr GeminiResponse
	if err := json.Unmarshal(chunk, &gr); err != nil {
		return nil, nil
	}
	sd := &StreamDelta{}
	for _, c := range gr.Candidates {
		for _, p := range c.Content.Parts {
			sd.Content += p.Text
		}
		if c.FinishReason != "" {
			fr := strings.ToLower(c.FinishReason)
			switch fr {
			case "stop":
				sd.FinishReason = "stop"
			case "max_tokens":
				sd.FinishReason = "length"
			case "safety":
				sd.FinishReason = "content_filter"
			default:
				sd.FinishReason = fr
			}
		}
	}
	return sd, nil
}

func marshalOpenAIStreamDelta(delta *StreamDelta) ([]byte, error) {
	if delta == nil {
		return nil, nil
	}
	chunk := OpenAIStreamChunk{
		Object: "chat.completion.chunk",
	}
	choice := OpenAIStreamChoice{
		Index: 0,
		Delta: OpenAIMessage{
			Role:      delta.Role,
			Content:   delta.Content,
			ToolCalls: delta.ToolCalls,
		},
	}
	if delta.FinishReason != "" {
		choice.FinishReason = &delta.FinishReason
	}
	chunk.Choices = []OpenAIStreamChoice{choice}

	data, err := json.Marshal(chunk)
	if err != nil {
		return nil, err
	}
	return []byte("data: " + string(data) + "\n\n"), nil
}

// ─── Helpers ──────────────────────────────────────────────────────────

func contentToString(c interface{}) string {
	if c == nil {
		return ""
	}
	switch v := c.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		b, _ := json.Marshal(c)
		return string(b)
	}
}
