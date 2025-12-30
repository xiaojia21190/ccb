package main

import (
	"bytes"
	"os/exec"
	"runtime"
	"strings"
)

// TerminalBackend 定义了终端操作的通用接口
type TerminalBackend interface {
	SendText(paneID string, text string) error
	IsAlive(paneID string) bool
	CreatePane(cmd string, cwd string, direction string) (string, error)
	CreatePaneAt(targetPaneID string, cmd string, cwd string, direction string) (string, error)
	KillPane(paneID string) error
	FocusPanel(panel string) error
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

func (w *WezTermBackend) FocusPanel(panel string) error {
	sess, err := LoadSession(panel)
	if err != nil {
		return err
	}
	paneID := sess.PaneID
	return exec.Command("wezterm", "cli", "activate-pane", "--pane-id", paneID).Run()
}

// 工厂函数
func GetBackend() TerminalBackend {
	return &WezTermBackend{}
}
