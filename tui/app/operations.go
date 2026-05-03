package app

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ayush1452/CLIverse/core/model"
)

type opResultMsg struct {
	success   bool
	message   string
	deletedID model.NodeID
}

func cmdCopyPath(path string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "linux":
			if _, err := exec.LookPath("xclip"); err == nil {
				cmd = exec.Command("xclip", "-selection", "clipboard")
			} else {
				cmd = exec.Command("xsel", "-ib")
			}
		case "windows":
			cmd = exec.Command("clip")
		default:
			return opResultMsg{false, "clipboard not supported on " + runtime.GOOS, 0}
		}
		in, err := cmd.StdinPipe()
		if err != nil {
			return opResultMsg{false, "clipboard error: " + err.Error(), 0}
		}
		if err := cmd.Start(); err != nil {
			return opResultMsg{false, "clipboard error: " + err.Error(), 0}
		}
		fmt.Fprint(in, path)
		in.Close()
		cmd.Wait()
		return opResultMsg{true, "Path copied to clipboard", 0}
	}
}

func cmdDeleteNode(path string, id model.NodeID, isDir bool) tea.Cmd {
	return func() tea.Msg {
		var err error
		if isDir {
			err = os.RemoveAll(path)
		} else {
			err = os.Remove(path)
		}
		if err != nil {
			return opResultMsg{false, "Delete failed: " + err.Error(), 0}
		}
		return opResultMsg{true, "Deleted: " + filepath.Base(path), id}
	}
}

func cmdRevealInFinder(path string) tea.Cmd {
	return func() tea.Msg {
		var err error
		switch runtime.GOOS {
		case "darwin":
			err = exec.Command("open", "-R", path).Start()
		case "linux":
			err = exec.Command("xdg-open", filepath.Dir(path)).Start()
		case "windows":
			err = exec.Command("explorer", "/select,"+path).Start()
		}
		if err != nil {
			return opResultMsg{false, "Reveal failed: " + err.Error(), 0}
		}
		return opResultMsg{true, "Opened in file manager", 0}
	}
}

func cmdOpenGUI(rootPath string) tea.Cmd {
	return func() tea.Msg {
		args := []string{"disk", "gui", rootPath}
		if err := exec.Command(os.Args[0], args...).Start(); err != nil {
			return opResultMsg{false, "GUI launch failed: " + err.Error(), 0}
		}
		return opResultMsg{true, "Browser GUI launched", 0}
	}
}
