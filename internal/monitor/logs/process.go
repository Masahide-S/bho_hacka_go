package logs

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// GetProcessLogs returns the last N lines of process logs from project directory
func GetProcessLogs(projectDir string, lines int) (string, error) {
	if projectDir == "" {
		return "", fmt.Errorf("プロジェクトディレクトリが見つかりません")
	}

	// 入力検証
	if strings.ContainsAny(projectDir, ";|&$`") {
		return "", fmt.Errorf("不正なディレクトリパスです")
	}

	// 一般的なログファイルのパターンを試す
	logPatterns := []string{
		filepath.Join(projectDir, "logs", "*.log"),
		filepath.Join(projectDir, "*.log"),
		filepath.Join(projectDir, ".log", "*.log"),
		filepath.Join(projectDir, "log", "*.log"),
	}

	// 固定ファイル名
	fixedLogFiles := []string{
		filepath.Join(projectDir, "npm-debug.log"),
		filepath.Join(projectDir, "yarn-error.log"),
	}

	// 最初に見つかったログファイルを使用
	var logFile string

	// パターンでGlob検索
	for _, pattern := range logPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			// 更新日時でソート（最新ファイルを優先）
			logFile = getNewestFile(matches)
			if logFile != "" {
				break
			}
		}
	}

	// パターンで見つからない場合、固定ファイル名を確認
	if logFile == "" {
		for _, file := range fixedLogFiles {
			if _, err := os.Stat(file); err == nil {
				logFile = file
				break
			}
		}
	}

	if logFile == "" {
		return "", fmt.Errorf("ログファイルが見つかりません")
	}

	// Goでファイルの最後のN行を読み込み
	content, err := readLastLines(logFile, lines)
	if err != nil {
		return "", fmt.Errorf("ログ取得失敗: %v", err)
	}

	return content, nil
}

// getNewestFile returns the newest file from the list based on modification time
func getNewestFile(files []string) string {
	if len(files) == 0 {
		return ""
	}

	type fileWithTime struct {
		path    string
		modTime int64
	}

	var filesWithTime []fileWithTime
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		filesWithTime = append(filesWithTime, fileWithTime{
			path:    f,
			modTime: info.ModTime().Unix(),
		})
	}

	if len(filesWithTime) == 0 {
		return ""
	}

	// 更新日時で降順ソート
	sort.Slice(filesWithTime, func(i, j int) bool {
		return filesWithTime[i].modTime > filesWithTime[j].modTime
	})

	return filesWithTime[0].path
}

// readLastLines reads the last N lines from a file
func readLastLines(filePath string, n int) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var allLines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	// 最後のN行を取得
	startIndex := 0
	if len(allLines) > n {
		startIndex = len(allLines) - n
	}

	lastLines := allLines[startIndex:]
	return strings.Join(lastLines, "\n"), nil
}
