package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// TerminalBackend 定义了终端操作的通用接口
type TerminalBackend interface {
	SendText(paneID string, text string) error
	IsAlive(paneID string) bool
	CreatePane(cmd string, cwd string, direction string) (string, error)
	CreatePaneAt(targetPaneID string, cmd string, cwd string, direction string) (string, error)
	KillPane(paneID string) error
}

// --- Tmux 实现 ---
type TmuxBackend struct{}

func (t *TmuxBackend) SendText(paneID string, text string) error {
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return nil
	}
	if err := exec.Command("tmux", "send-keys", "-t", paneID, "-l", cleanText).Run(); err != nil {
		return err
	}
	return exec.Command("tmux", "send-keys", "-t", paneID, "Enter").Run()
}

func (t *TmuxBackend) IsAlive(paneID string) bool {
	err := exec.Command("tmux", "has-session", "-t", paneID).Run()
	return err == nil
}

func (t *TmuxBackend) CreatePane(cmd string, cwd string, direction string) (string, error) {
	sessionName := fmt.Sprintf("ai-%d", time.Now().UnixNano())
	// -d: detached, -s: session name, -c: cwd
	c := exec.Command("tmux", "new-session", "-d", "-s", sessionName, "-c", cwd, cmd)
	if err := c.Run(); err != nil {
		return "", err
	}
	return sessionName, nil
}

func (t *TmuxBackend) KillPane(paneID string) error {
	// 对于 Tmux，paneID 就是 session name
	return exec.Command("tmux", "kill-session", "-t", paneID).Run()
}

func (t *TmuxBackend) CreatePaneAt(targetPaneID string, cmd string, cwd string, direction string) (string, error) {
	// Tmux 暂不支持，使用普通创建
	return t.CreatePane(cmd, cwd, direction)
}

// --- WezTerm 实现 ---
type WezTermBackend struct{}

func (w *WezTermBackend) SendText(paneID string, text string) error {
	cleanText := strings.TrimSpace(text)
	if cleanText == "" {
		return nil
	}
	if err := exec.Command("wezterm", "cli", "send-text", "--pane-id", paneID, "--no-paste", cleanText).Run(); err != nil {
		return err
	}
	return exec.Command("wezterm", "cli", "send-text", "--pane-id", paneID, "--no-paste", "\r").Run()
}

func (w *WezTermBackend) IsAlive(paneID string) bool {
	var out bytes.Buffer
	cmd := exec.Command("wezterm", "cli", "list")
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return false
	}
	return strings.Contains(out.String(), paneID)
}

func (w *WezTermBackend) CreatePane(cmd string, cwd string, direction string) (string, error) {
	args := []string{"cli", "split-pane", "--cwd", cwd}
	if direction == "right" {
		args = append(args, "--right")
	} else {
		args = append(args, "--bottom")
	}
	args = append(args, "--percent", "50", "--")

	// Windows 使用 cmd /c，Linux/macOS 使用 bash -c
	if runtime.GOOS == "windows" {
		args = append(args, "cmd", "/c", cmd)
	} else {
		args = append(args, "bash", "-c", cmd)
	}

	out, err := exec.Command("wezterm", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func (w *WezTermBackend) KillPane(paneID string) error {
	return exec.Command("wezterm", "cli", "kill-pane", "--pane-id", paneID).Run()
}

func (w *WezTermBackend) CreatePaneAt(targetPaneID string, cmd string, cwd string, direction string) (string, error) {
	args := []string{"cli", "split-pane", "--pane-id", targetPaneID, "--cwd", cwd}
	switch direction {
	case "left":
		args = append(args, "--left")
	case "right":
		args = append(args, "--right")
	case "top":
		args = append(args, "--top")
	default:
		args = append(args, "--bottom")
	}
	args = append(args, "--percent", "50", "--")

	if runtime.GOOS == "windows" {
		args = append(args, "cmd", "/c", cmd)
	} else {
		args = append(args, "bash", "-c", cmd)
	}

	out, err := exec.Command("wezterm", args...).Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// 工厂函数
func GetBackend(name string) TerminalBackend {
	if name == "wezterm" {
		return &WezTermBackend{}
	}
	return &TmuxBackend{}
}
