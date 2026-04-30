// Package sysmon provides an HTTP/SSE server that streams system metrics
// to a browser-based ECharts dashboard.
package sysmon

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	core "github.com/ayush1452/CLIverse/core/sysmon"
)

//go:embed frontend
var frontendFiles embed.FS

// Server streams live metrics over HTTP/SSE and accepts control actions.
type Server struct {
	collector *core.Collector
	port      int
}

// NewServer allocates a Server on a random free port.
func NewServer() (*Server, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	return &Server{collector: core.New(), port: port}, nil
}

// URL returns the http://localhost:PORT base URL.
func (s *Server) URL() string {
	return fmt.Sprintf("http://localhost:%d", s.port)
}

// Start binds the HTTP listener. Blocks until the server exits.
func (s *Server) Start() error {
	sub, err := fs.Sub(frontendFiles, "frontend")
	if err != nil {
		return err
	}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/events", s.handleEvents)
	mux.HandleFunc("/kill", s.handleKill)

	return http.ListenAndServe(fmt.Sprintf("localhost:%d", s.port), mux)
}

// handleEvents writes a Server-Sent Events stream of JSON snapshots.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			snap := s.collector.Collect()
			b, err := json.Marshal(snap)
			if err != nil {
				continue
			}
			fmt.Fprintf(w, "data: %s\n\n", b)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// handleKill accepts {"pid": N} and sends SIGKILL to that process.
func (s *Server) handleKill(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PID int `json:"pid"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PID <= 1 {
		http.Error(w, "invalid pid", http.StatusBadRequest)
		return
	}
	proc, err := os.FindProcess(req.PID)
	if err != nil {
		http.Error(w, "process not found", http.StatusNotFound)
		return
	}
	if err := proc.Kill(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "killed"})
}

// OpenBrowser opens url in the system default browser.
func OpenBrowser(url string) {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd, args = "open", []string{url}
	case "linux":
		cmd, args = "xdg-open", []string{url}
	case "windows":
		cmd, args = "cmd", []string{"/c", "start", "", url}
	default:
		return
	}
	exec.Command(cmd, args...).Start() //nolint:errcheck
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
