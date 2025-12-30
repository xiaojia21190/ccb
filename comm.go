package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

	// 递归查找所有匹配文件（Codex 和 Gemini 都有嵌套目录结构）
	err = filepath.Walk(fullDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略访问错误
		}
		if !info.IsDir() && strings.Contains(info.Name(), pattern) {
			matches = append(matches, path)
		}
		return nil
	})

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

// 获取 Codex 历史记录
func GetCodexHistory(n int) ([]string, error) {
	logPath, err := findLatestLog(".codex/sessions", "rollout-")
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
