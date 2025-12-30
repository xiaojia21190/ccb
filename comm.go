package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// 通用工具：查找最新的日志文件
// dir: 相对于用户主目录的路径 (例如 ".codex/sessions")
// pattern: 文件后缀匹配 (例如 ".jsonl")
func findLatestLog(dir string, pattern string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	fullDir := filepath.Join(home, dir)

	var matches []string

	// 如果目录不存在，直接返回错误
	if _, err := os.Stat(fullDir); os.IsNotExist(err) {
		return "", fmt.Errorf("directory not found: %s", fullDir)
	}

	// 针对 Gemini 复杂的嵌套目录结构进行递归查找
	// 结构通常是: ~/.gemini/tmp/<hash>/chats/<session>.json
	if strings.Contains(dir, "gemini") {
		err = filepath.Walk(fullDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // 忽略访问错误
			}
			if !info.IsDir() && strings.HasSuffix(info.Name(), pattern) {
				matches = append(matches, path)
			}
			return nil
		})
	} else {
		// Codex 结构较平坦: ~/.codex/sessions/*.jsonl
		entries, err := os.ReadDir(fullDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() && strings.HasSuffix(e.Name(), pattern) {
					matches = append(matches, filepath.Join(fullDir, e.Name()))
				}
			}
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no logs found in %s", fullDir)
	}

	// 按修改时间排序，取最新的
	sort.Slice(matches, func(i, j int) bool {
		fi, _ := os.Stat(matches[i])
		fj, _ := os.Stat(matches[j])
		// fi, fj 可能有 error，这里简单忽略，假定文件存在
		if fi == nil || fj == nil {
			return false
		}
		return fi.ModTime().Before(fj.ModTime())
	})

	return matches[len(matches)-1], nil
}

// --- Codex 相关逻辑 ---

// 等待 Codex 回复 (针对追加写入的 JSONL 文件)
func WaitCodexReply(timeout time.Duration) (string, error) {
	logPath, err := findLatestLog(".codex/sessions", ".jsonl")
	if err != nil {
		return "", err
	}

	file, err := os.Open(logPath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	// 1. 移动到文件末尾 (Seek End)，只监听新内容
	file.Seek(0, io.SeekEnd)
	reader := bufio.NewReader(file)

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		// 尝试读取一行
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// 没新内容，稍等一下再读
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return "", err
		}

		// 解析新行
		var entry CodexEntry
		if err := json.Unmarshal([]byte(line), &entry); err == nil {
			if entry.Type == "response_item" {
				var payload CodexResponsePayload
				if err := json.Unmarshal(entry.Payload, &payload); err == nil {
					// 优先检查新版 content 字段
					var sb strings.Builder
					if len(payload.Content) > 0 {
						for _, c := range payload.Content {
							if c.Type == "output_text" {
								sb.WriteString(c.Text)
							}
						}
					}
					// 如果 content 为空，回退到旧版 message 字段
					if sb.Len() == 0 && payload.Message != "" {
						sb.WriteString(payload.Message)
					}

					if sb.Len() > 0 {
						return sb.String(), nil
					}
				}
			}
		}
	}
	return "", fmt.Errorf("timeout waiting for Codex reply")
}

// 获取 Codex 历史记录
func GetCodexHistory(n int) ([]string, error) {
	logPath, err := findLatestLog(".codex/sessions", ".jsonl")
	if err != nil {
		return nil, err
	}

	file, err := os.Open(logPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 增加 Buffer 大小以应对长行
	const maxCapacity = 1024 * 1024 // 1MB
	buf := make([]byte, maxCapacity)
	scanner.Buffer(buf, maxCapacity)

	var qaPairs []string
	// 这里简化处理：我们只提取 Codex 的回复作为一条记录
	// 完整的 Q&A 匹配需要更复杂的状态机逻辑，因为 input 和 output 是分行的

	for scanner.Scan() {
		line := scanner.Bytes()
		var entry CodexEntry
		if json.Unmarshal(line, &entry) != nil {
			continue
		}

		if entry.Type == "response_item" {
			var pl CodexResponsePayload
			json.Unmarshal(entry.Payload, &pl)

			text := pl.Message
			if text == "" {
				for _, c := range pl.Content {
					if c.Type == "output_text" {
						text += c.Text
					}
				}
			}
			if strings.TrimSpace(text) != "" {
				// 这里简单地把每一条回复当作一个历史项
				qaPairs = append(qaPairs, fmt.Sprintf("A: %s", strings.TrimSpace(text)))
			}
		}
	}

	if n > len(qaPairs) {
		n = len(qaPairs)
	}
	if n == 0 {
		return []string{}, nil
	}
	return qaPairs[len(qaPairs)-n:], nil
}

// --- Gemini 相关逻辑 ---

// 等待 Gemini 回复 (针对全量重写的 JSON 文件)
func WaitGeminiReply(timeout time.Duration) (string, error) {
	logPath, err := findLatestLog(".gemini/tmp", ".json")
	if err != nil {
		// 尝试标准路径作为 fallback
		logPath, err = findLatestLog(".gemini/chats", ".json")
		if err != nil {
			return "", err
		}
	}

	lastCount := 0
	deadline := time.Now().Add(timeout)

	// 1. 获取初始状态
	if content, err := os.ReadFile(logPath); err == nil {
		var session GeminiSession
		if json.Unmarshal(content, &session) == nil {
			lastCount = len(session.Messages)
		}
	}

	// 2. 轮询检查
	for time.Now().Before(deadline) {
		time.Sleep(200 * time.Millisecond)

		content, err := os.ReadFile(logPath)
		if err != nil {
			continue
		}

		var session GeminiSession
		if err := json.Unmarshal(content, &session); err != nil {
			continue
		}

		if len(session.Messages) > lastCount {
			// 发现新消息
			newMsgs := session.Messages[lastCount:]
			// 找到最后一条 gemini 的回复
			for i := len(newMsgs) - 1; i >= 0; i-- {
				msg := newMsgs[i]
				if msg.Type == "gemini" || msg.Type == "model" { // 兼容 type 名称
					var text string
					// Content 可能是 string 也可能是对象
					if err := json.Unmarshal(msg.Content, &text); err == nil {
						return text, nil
					}
					// 如果解析 string 失败，可能是复杂对象，返回 JSON string 作为 fallback
					return string(msg.Content), nil
				}
			}
			lastCount = len(session.Messages)
		}
	}
	return "", fmt.Errorf("timeout waiting for Gemini reply")
}

// 获取 Gemini 历史记录
func GetGeminiHistory(n int) ([]string, error) {
	logPath, err := findLatestLog(".gemini/tmp", ".json")
	if err != nil {
		// Fallback
		logPath, err = findLatestLog(".gemini/chats", ".json")
		if err != nil {
			return nil, err
		}
	}

	content, err := os.ReadFile(logPath)
	if err != nil {
		return nil, err
	}

	var session GeminiSession
	if err := json.Unmarshal(content, &session); err != nil {
		return nil, err
	}

	var qaPairs []string
	var currentQ string

	for _, msg := range session.Messages {
		var text string
		if err := json.Unmarshal(msg.Content, &text); err != nil {
			// 简化处理复杂对象
			text = "[Complex Content]"
		}
		text = strings.TrimSpace(text)

		switch msg.Type {
		case "user":
			currentQ = text
		case "gemini", "model":
			pair := fmt.Sprintf("A: %s", text)
			if currentQ != "" {
				pair = fmt.Sprintf("Q: %s\n%s", currentQ, pair)
				currentQ = "" // Reset
			}
			qaPairs = append(qaPairs, pair)
		}
	}

	if n > len(qaPairs) {
		n = len(qaPairs)
	}
	if n == 0 {
		return []string{}, nil
	}
	return qaPairs[len(qaPairs)-n:], nil
}
