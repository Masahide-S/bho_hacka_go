package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

var logDir string

// InitLogger initializes the logger
func InitLogger() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	logDir = filepath.Join(home, ".devmon", "logs")

	// ディレクトリ作成
	return os.MkdirAll(logDir, 0755)
}

// LogSystemResources logs system resources
func LogSystemResources(cpu float64, memUsed, memTotal int64) {
	logFile := filepath.Join(logDir, fmt.Sprintf("system_%s.log", time.Now().Format("2006-01-02")))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] CPU: %.1f%% | Memory: %dMB/%dMB\n",
		timestamp, cpu, memUsed, memTotal)

	f.WriteString(line)
}

// LogServiceStatus logs service status change
func LogServiceStatus(serviceName, status string) {
	logFile := filepath.Join(logDir, fmt.Sprintf("services_%s.log", time.Now().Format("2006-01-02")))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s: %s\n", timestamp, serviceName, status)

	f.WriteString(line)
}

// LogIssue logs detected issue
func LogIssue(issueType, description string) {
	logFile := filepath.Join(logDir, fmt.Sprintf("issues_%s.log", time.Now().Format("2006-01-02")))

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] [%s] %s\n", timestamp, issueType, description)

	f.WriteString(line)
}
