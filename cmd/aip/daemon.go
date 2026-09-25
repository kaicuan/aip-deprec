package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"aip/internal/config"
	"aip/internal/db"
)

func isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	// On Unix, signal 0 does error-checking without killing the process
	err = process.Signal(syscall.Signal(0))
	return err == nil
}

func getRunningPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}
	pidStr := strings.TrimSpace(string(data))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		return 0, err
	}
	if !isProcessRunning(pid) {
		_ = os.Remove(pidFile)
		return 0, fmt.Errorf("stale PID %d", pid)
	}
	return pid, nil
}

func startDaemon(cfg *config.Config) error {
	pid, err := getRunningPID(cfg.PIDPath)
	if err == nil && pid > 0 {
		fmt.Printf("AIP Gateway is already running (PID: %d) on http://localhost:%d\n", pid, cfg.Port)
		return nil
	}

	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	logFile, err := os.OpenFile(cfg.LogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	cmd := exec.Command(execPath, "__run_daemon")
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("failed to start daemon process: %w", err)
	}

	childPID := cmd.Process.Pid
	if err := os.WriteFile(cfg.PIDPath, []byte(strconv.Itoa(childPID)), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Give process a moment to initialize
	time.Sleep(300 * time.Millisecond)
	if !isProcessRunning(childPID) {
		_ = os.Remove(cfg.PIDPath)
		return fmt.Errorf("daemon failed to start. Check %s for details", cfg.LogPath)
	}

	fmt.Printf("AIP Gateway started successfully (PID: %d)\n", childPID)
	fmt.Printf("Dashboard & Gateway: http://localhost:%d\n", cfg.Port)
	fmt.Printf("Logs: %s\n", cfg.LogPath)
	return nil
}

func stopDaemon(cfg *config.Config) error {
	pid, err := getRunningPID(cfg.PIDPath)
	if err != nil || pid <= 0 {
		fmt.Println("AIP Gateway is not running.")
		_ = os.Remove(cfg.PIDPath)
		return nil
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		_ = os.Remove(cfg.PIDPath)
		fmt.Println("AIP Gateway is not running.")
		return nil
	}

	fmt.Printf("Stopping AIP Gateway (PID: %d)...\n", pid)
	_ = process.Signal(syscall.SIGTERM)

	// Wait up to 5 seconds for graceful stop
	stopped := false
	for i := 0; i < 50; i++ {
		time.Sleep(100 * time.Millisecond)
		if !isProcessRunning(pid) {
			stopped = true
			break
		}
	}

	if !stopped {
		fmt.Println("Force killing process...")
		_ = process.Kill()
	}

	_ = os.Remove(cfg.PIDPath)
	fmt.Println("AIP Gateway stopped.")
	return nil
}

func daemonStatus(cfg *config.Config) error {
	pid, err := getRunningPID(cfg.PIDPath)
	if err != nil || pid <= 0 {
		fmt.Println("Status: STOPPED")
		fmt.Println("Run 'aip start' to launch the gateway.")
		return nil
	}

	// Gateway is running, let's query its health endpoint for accurate runtime stats
	healthURL := fmt.Sprintf("http://localhost:%d/api/health", cfg.Port)
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(healthURL)
	if err == nil && resp.StatusCode == http.StatusOK {
		var stats struct {
			Status        string  `json:"status"`
			ActiveProxies int64   `json:"active_proxies"`
			TotalRequests int64   `json:"total_requests"`
			SuccessRate   float64 `json:"success_rate"`
			Uptime        string  `json:"uptime"`
		}
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		_ = json.Unmarshal(body, &stats)

		fmt.Println("Status: RUNNING")
		fmt.Printf("PID:            %d\n", pid)
		fmt.Printf("Endpoint:       http://localhost:%d\n", cfg.Port)
		if stats.Uptime != "" {
			fmt.Printf("Uptime:         %s\n", stats.Uptime)
		}
		fmt.Printf("Active Proxies: %d\n", stats.ActiveProxies)
		fmt.Printf("Total Requests: %d\n", stats.TotalRequests)
		fmt.Printf("Success Rate:   %.1f%%\n", stats.SuccessRate)
		return nil
	}

	// Fallback to reading DB directly
	database, dbErr := db.Open(cfg.DBPath)
	if dbErr == nil {
		defer database.Close()
		st, _ := database.GetStats()
		fmt.Println("Status: RUNNING")
		fmt.Printf("PID:            %d\n", pid)
		fmt.Printf("Endpoint:       http://localhost:%d\n", cfg.Port)
		if st != nil {
			fmt.Printf("Active Proxies: %d / %d\n", st.ActiveProxiesCount, st.TotalProxiesCount)
			fmt.Printf("Total Requests: %d\n", st.TotalRequests)
			fmt.Printf("Success Rate:   %.1f%%\n", st.SuccessRate)
		}
		return nil
	}

	fmt.Println("Status: RUNNING")
	fmt.Printf("PID:      %d\n", pid)
	fmt.Printf("Endpoint: http://localhost:%d\n", cfg.Port)
	return nil
}
