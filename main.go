package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func main() {
	// 获取当前执行文件名
	execPath, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting executable path: %v\n", err)
		os.Exit(1)
	}
	execName := filepath.Base(execPath)
	execName = strings.TrimSuffix(execName, ".exe")

	// 多路复用路由 (Multi-call binary)
	switch execName {
	case "cask":
		runAsyncAsk("codex")
	case "gask":
		runAsyncAsk("gemini")
	case "cask-w":
		runSyncAsk("codex")
	case "gask-w":
		runSyncAsk("gemini")
	case "cpend":
		runPend("codex")
	case "gpend":
		runPend("gemini")
	case "cping":
		runPing("codex")
	case "gping":
		runPing("gemini")
	default:
		// ccb 主入口
		if len(os.Args) < 2 {
			printHelp()
			return
		}
		switch os.Args[1] {
		case "up":
			runUp(os.Args[2:])
		case "kill":
			runKill(os.Args[2:])
		case "status":
			runStatus()
		case "install":
			runInstall(execPath)
		case "help":
			printHelp()
		default:
			printHelp()
		}
	}
}

// --- 命令实现 ---

// 0. ccb install
func runInstall(currentPath string) {
	dir := filepath.Dir(currentPath)
	links := []string{"cask", "gask", "cask-w", "gask-w", "cpend", "gpend", "cping", "gping"}

	fmt.Printf("Installing symlinks in %s...\n", dir)

	for _, linkName := range links {
		linkPath := filepath.Join(dir, linkName)
		// 如果文件已存在，先删除
		if _, err := os.Lstat(linkPath); err == nil {
			os.Remove(linkPath)
		}

		// 创建软链接
		target := filepath.Base(currentPath)
		err := os.Symlink(target, linkPath)
		if err != nil {
			fmt.Printf("❌ Failed to create %s: %v\n", linkName, err)
		} else {
			fmt.Printf("✅ Created %s\n", linkName)
		}
	}
	fmt.Println("Installation complete.")
}

// 1. ccb up [claude] [codex] [gemini]
func runUp(args []string) {
	if len(args) == 0 {
		fmt.Println("Usage: ccb up <provider>...")
		return
	}

	currentPaneID := os.Getenv("WEZTERM_PANE")
	backend := GetBackend()

	cwd, _ := os.Getwd()

	// 检查要启动的服务
	hasCodex, hasGemini, hasClaude := false, false, true
	for _, p := range args {
		switch p {
		case "codex":
			hasCodex = true
		case "gemini":
			hasGemini = true
		}
	}

	var codexPaneID string

	// 1. Codex 在下方
	if hasCodex {
		fmt.Println("Starting codex...")
		paneID, err := backend.CreatePane("codex -c disable_paste_burst=true", cwd, "bottom")
		if err != nil {
			fmt.Printf("Error starting codex: %v\n", err)
		} else {
			codexPaneID = paneID
			time.Sleep(500 * time.Millisecond)
			sess := &LocalSession{
				SessionID: fmt.Sprintf("codex-%d", time.Now().Unix()),
				PaneID:    paneID,
				Active:    true,
				WorkDir:   cwd,
				StartedAt: time.Now().Format(time.RFC3339),
			}
			SaveSession("codex", sess)
			fmt.Printf("✅ codex started. ID: %s\n", paneID)
		}
	}

	// 2. Gemini 在 Codex 右边
	if hasGemini {
		fmt.Println("Starting gemini...")
		paneID, err := backend.CreatePaneAt(codexPaneID, "gemini", cwd, "right")
		if err != nil {
			fmt.Printf("Error starting gemini: %v\n", err)
		} else {
			time.Sleep(500 * time.Millisecond)
			sess := &LocalSession{
				SessionID: fmt.Sprintf("gemini-%d", time.Now().Unix()),
				PaneID:    paneID,
				Active:    true,
				WorkDir:   cwd,
				StartedAt: time.Now().Format(time.RFC3339),
			}
			SaveSession("gemini", sess)
			fmt.Printf("✅ gemini started. ID: %s\n", paneID)
		}
	}

	// 3. Claude 在最下方（占满宽度）
	if hasClaude {
		fmt.Println("Starting claude...")
		paneID, err := backend.CreatePane("claude", cwd, "bottom")
		if err != nil {
			fmt.Printf("Error starting claude: %v\n", err)
		} else {
			time.Sleep(500 * time.Millisecond)
			sess := &LocalSession{
				SessionID: fmt.Sprintf("claude-%d", time.Now().Unix()),
				PaneID:    paneID,
				Active:    true,
				WorkDir:   cwd,
				StartedAt: time.Now().Format(time.RFC3339),
			}
			SaveSession("claude", sess)
			fmt.Printf("✅ claude started. ID: %s\n", paneID)
		}
	}

	// 关闭运行命令的原始窗格
	if currentPaneID != "" && (hasCodex || hasGemini || hasClaude) {
		time.Sleep(300 * time.Millisecond)
		exec.Command("wezterm", "cli", "kill-pane", "--pane-id", currentPaneID).Run()
	}

	//光标转移到claude窗格
	if hasClaude {
		backend.FocusPanel("claude")
	}
}

// 2. ccb kill [claude] [codex] [gemini]
func runKill(args []string) {
	if len(args) == 0 {
		args = []string{"claude", "codex", "gemini"}
	}

	for _, provider := range args {
		sess, err := LoadSession(provider)
		if err != nil {
			fmt.Printf("Info: %s session not found or inactive.\n", provider)
			continue
		}

		backend := GetBackend()
		id := sess.PaneID

		// 调用 KillPane
		if err := backend.KillPane(id); err != nil {
			fmt.Printf("Warning: failed to kill pane for %s: %v\n", provider, err)
		}

		TerminateSession(provider)
		fmt.Printf("Killed %s\n", provider)
	}
}

// 3. ccb status
func runStatus() {
	for _, p := range []string{"claude", "codex", "gemini"} {
		sess, err := LoadSession(p)
		status := "Stopped"
		if err == nil {
			backend := GetBackend()
			id := sess.PaneID

			if backend.IsAlive(id) {
				status = fmt.Sprintf("Running (pane: %s)", id)
			} else {
				status = "Dead (Process missing)"
			}
		}
		fmt.Printf("%-10s: %s\n", p, status)
	}
}

// 4. cask/gask (Async)
func runAsyncAsk(provider string) {
	fs := flag.NewFlagSet(provider, flag.ExitOnError)
	fs.Parse(os.Args[1:])
	msg := strings.Join(fs.Args(), " ")

	if msg == "" {
		fmt.Printf("Usage: %s <message>\n", provider)
		os.Exit(1)
	}

	sess, err := LoadSession(provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s session not active. Run 'ccb up %s' first.\n", provider, provider)
		os.Exit(1)
	}

	backend := GetBackend()
	id := sess.PaneID

	if err := backend.SendText(id, msg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Sent.")
}

// 5. cask-w/gask-w (Sync)
func runSyncAsk(provider string) {
	var timeout int
	var output string
	fs := flag.NewFlagSet(provider+"-w", flag.ExitOnError)
	fs.IntVar(&timeout, "timeout", 60, "timeout")
	fs.StringVar(&output, "output", "", "output file")
	fs.Parse(os.Args[1:])
	msg := strings.Join(fs.Args(), " ")

	if msg == "" {
		fmt.Printf("Usage: %s <message>\n", os.Args[0])
		os.Exit(1)
	}

	sess, err := LoadSession(provider)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: Session not active. Run 'ccb up %s' first.\n", provider)
		os.Exit(1)
	}
	backend := GetBackend()
	id := sess.PaneID

	// Send
	if err := backend.SendText(id, msg); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to send: %v\n", err)
		os.Exit(1)
	}

	// Wait
	var reply string
	if provider == "codex" {
		reply, err = WaitCodexReply(time.Duration(timeout) * time.Second)
	} else {
		reply, err = WaitGeminiReply(time.Duration(timeout) * time.Second)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Timeout or Error: %v\n", err)
		os.Exit(2)
	}

	if output != "" {
		os.WriteFile(output, []byte(reply), 0644)
	} else {
		fmt.Println(reply)
	}
}

// 6. cpend/gpend
func runPend(provider string) {
	n := 1
	if len(os.Args) > 1 {
		if val, err := strconv.Atoi(os.Args[1]); err == nil {
			n = val
		}
	}

	var history []string
	var err error
	if provider == "codex" {
		history, err = GetCodexHistory(n)
	} else {
		history, err = GetGeminiHistory(n)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "No logs found: %v\n", err)
		os.Exit(2)
	}

	for i, item := range history {
		fmt.Println(item)
		if i < len(history)-1 {
			fmt.Println("---")
		}
	}
}

// 7. cping/gping
func runPing(provider string) {
	sess, err := LoadSession(provider)
	if err != nil {
		fmt.Printf("❌ %s: Session info not found (run 'ccb up %s')\n", provider, provider)
		os.Exit(1)
	}

	backend := GetBackend()
	id := sess.PaneID

	if backend.IsAlive(id) {
		fmt.Printf("✅ %s connection OK (pane: %s)\n", provider, id)
	} else {
		fmt.Printf("❌ %s: Process dead but session file exists\n", provider)
		os.Exit(1)
	}
}

func printHelp() {
	fmt.Println(`Claude Code Bridge (Go Edition)

Usage:
  ccb <command> [args]

Commands:
  install                Create symlinks (cask, gask...) in current dir
  up <provider>...       Start backends (claude, codex, gemini)
  kill <provider>...     Stop backends and close panes
  status                 Show backend status

  cask <msg>             Send to Codex (Async)
  cask-w <msg>           Send to Codex and wait
  cpend [N]              View Codex history (last N items)
  cping                  Check Codex connection

  gask <msg>             Send to Gemini (Async)
  gask-w <msg>           Send to Gemini and wait
  gpend [N]              View Gemini history
  gping                  Check Gemini connection`)
}
