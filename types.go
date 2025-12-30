package main

import "encoding/json"

// Codex Log 结构
type CodexEntry struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

type CodexResponsePayload struct {
	Type    string         `json:"type"`
	Message string         `json:"message"` // Legacy
	Content []CodexContent `json:"content"` // New
}

type CodexContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Gemini Log 结构
type GeminiSession struct {
	Messages []GeminiMessage `json:"messages"`
}

type GeminiMessage struct {
	Type    string          `json:"type"` // "gemini" or "user"
	Content json.RawMessage `json:"content"`
}

// 本地会话状态 (.codex-session / .gemini-session)
type LocalSession struct {
	SessionID string `json:"session_id"`
	PaneID    string `json:"pane_id"`
	Active    bool   `json:"active"`
	WorkDir   string `json:"work_dir"`
	StartedAt string `json:"started_at"`
}
