package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// 获取 Session 文件路径
// 改进：优先存储在 ~/.ccb/ 目录下，如果不存在则创建，确保全局访问
func getSessionPath(provider string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		// Fallback to CWD if home not found
		cwd, _ := os.Getwd()
		return filepath.Join(cwd, fmt.Sprintf(".%s-session", provider))
	}

	// 创建隐藏配置目录
	configDir := filepath.Join(home, ".ccb")
	if _, err := os.Stat(configDir); os.IsNotExist(err) {
		os.MkdirAll(configDir, 0755)
	}

	return filepath.Join(configDir, fmt.Sprintf("%s-session.json", provider))
}

func LoadSession(provider string) (*LocalSession, error) {
	path := getSessionPath(provider)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var s LocalSession
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	if !s.Active {
		return nil, fmt.Errorf("session inactive")
	}
	return &s, nil
}

func SaveSession(provider string, s *LocalSession) error {
	path := getSessionPath(provider)
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func TerminateSession(provider string) error {
	s, err := LoadSession(provider)
	if err != nil {
		return nil
	}
	s.Active = false
	return SaveSession(provider, s)
}
