package llm

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/Masahide-S/bho_hacka_go/internal/monitor"
)

// ==========================================
// 1. Data Structures (Context Definitions)
// ==========================================

// SystemContext はシステムリソース情報を保持します
type SystemContext struct {
	CPUUsage     float64               `json:"cpu_usage"`
	MemoryUsed   int64                 `json:"memory_used_mb"`
	MemoryTotal  int64                 `json:"memory_total_mb"`
	MemoryPerc   float64               `json:"memory_usage_percent"`
	DiskUsage    float64               `json:"disk_usage_percent"`  // 新規追加
	DiskFreeGB   float64               `json:"disk_free_gb"`        // 新規追加
	TopProcesses []monitor.ProcessInfo `json:"top_processes"`
}

// DockerContainer は個々のコンテナ情報を保持します
type DockerContainer struct {
	ID      string `json:"id"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	Names   string `json:"names"`
	Ports   string `json:"ports"`
	MemUsed string `json:"mem_used"` // docker statsから取得
	CPUUsed string `json:"cpu_used"` // docker statsから取得
	// ▼ 障害診断用の詳細メタデータ（docker inspect から取得）
	ExitCode  int    `json:"exit_code"`
	OOMKilled bool   `json:"oom_killed"`
	Error     string `json:"error"` // Dockerが吐くエラーメッセージがあれば
}

// DockerContext はDocker関連情報を構造化して保持します
type DockerContext struct {
	IsRunning  bool              `json:"is_running"`
	Count      int               `json:"container_count"`
	Containers []DockerContainer `json:"containers"`
}

// ProcessContext はランタイム情報を保持します
type ProcessContext struct {
	NodeProcess    ProcessDetail `json:"node"`
	PythonProcess  ProcessDetail `json:"python"`
	ListeningPorts []PortInfo    `json:"listening_ports"`
}

type ProcessDetail struct {
	Detected bool   `json:"detected"`
	Pid      string `json:"pid"`
	Command  string `json:"command"`
	Version  string `json:"version"` // 可能なら取得
}

type PortInfo struct {
	Port    string `json:"port"`
	Process string `json:"process"`
	Pid     string `json:"pid"`
}

// DatabaseContext はDB情報を保持します
type DatabaseContext struct {
	Postgres DBStatus `json:"postgres"`
	MySQL    DBStatus `json:"mysql"`
	Redis    DBStatus `json:"redis"`
}

type DBStatus struct {
	IsRunning bool   `json:"is_running"`
	Port      string `json:"port"`
	Message   string `json:"message"` // エラーや状態の詳細
}

// ProjectInfo は個別のプロジェクト情報を保持します
type ProjectInfo struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"` // "Node.js", "Go", "Python" etc.
	Path         string   `json:"path"`
	Dependencies []string `json:"dependencies"` // 主要な依存ライブラリ
}

// ProjectContext はプロジェクト全体の情報を保持します
type ProjectContext struct {
	CollectedAt time.Time     `json:"collected_at"`
	Projects    []ProjectInfo `json:"projects"`
	CurrentDir  string        `json:"current_dir"`
}

// FullContext は全ての情報を統合した構造体です
type FullContext struct {
	System   *SystemContext   `json:"system"`
	Docker   *DockerContext   `json:"docker"`
	Process  *ProcessContext  `json:"process"`
	Database *DatabaseContext `json:"database"`
	Project  *ProjectContext  `json:"project"`
}

// ==========================================
// 2. Collection Logic
// ==========================================

// CollectSystemContext はシステム情報を収集します
func CollectSystemContext() (*SystemContext, error) {
	// 既存のmonitorパッケージを利用
	resources := monitor.GetSystemResources()
	topProcs := monitor.GetTopProcesses(5)

	// ディスク使用率の取得 (dfコマンドを使用)
	diskUsage, freeGB := getDiskUsage()

	return &SystemContext{
		CPUUsage:     resources.CPUUsage,
		MemoryUsed:   resources.MemoryUsed,
		MemoryTotal:  resources.MemoryTotal,
		MemoryPerc:   resources.MemoryPerc,
		DiskUsage:    diskUsage,
		DiskFreeGB:   freeGB,
		TopProcesses: topProcs,
	}, nil
}

func getDiskUsage() (float64, float64) {
	// Unix系向け dfコマンド実行
	cmd := exec.Command("df", "-h", "/")
	if runtime.GOOS == "windows" {
		// Windows対応が必要な場合は別途実装 (今回は省略)
		return 0, 0
	}

	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	// dfの出力を解析 (行ごとに処理)
	lines := strings.Split(string(output), "\n")
	if len(lines) < 2 {
		return 0, 0
	}

	fields := strings.Fields(lines[1])
	// Filesystem Size Used Avail Capacity iused ifree %iused Mounted on
	// 通常、Capacityは5番目(インデックス4)か、Availは4番目(インデックス3)
	// Mac/Linuxで出力形式が多少異なるが、"%"が含まれるフィールドを探すのが確実

	var usage float64
	var freeGB float64

	for _, f := range fields {
		if strings.Contains(f, "%") {
			val := strings.TrimSuffix(f, "%")
			u, _ := strconv.ParseFloat(val, 64)
			usage = u
		}
		// Avail (Gi, G, Mなど単位付き) の簡易パースは複雑なため、
		// 今回は % から逆算せず、安全策として使用率のみ優先で返す設計も可だが、
		// 簡易的に実装する
	}

	return usage, freeGB
}

// CollectDockerContext はDocker情報をコマンドから直接収集します
func CollectDockerContext() (*DockerContext, error) {
	// docker ps -a でIDを取得（-a オプションで停止コンテナも取得）
	cmd := exec.Command("docker", "ps", "-a", "--format", "{{.ID}}|{{.Image}}|{{.Status}}|{{.Names}}|{{.Ports}}")
	output, err := cmd.Output()

	// Dockerが起動していない場合
	if err != nil {
		return &DockerContext{IsRunning: false}, nil
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return &DockerContext{IsRunning: true, Count: 0, Containers: []DockerContainer{}}, nil
	}

	var containers []DockerContainer
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) < 5 {
			continue
		}

		c := DockerContainer{
			ID:     parts[0],
			Image:  parts[1],
			Status: parts[2],
			Names:  parts[3],
			Ports:  parts[4],
		}

		// リソース情報の取得 (docker stats --no-stream)
		// 実行中の場合のみ取得（停止コンテナはstatsが取れない）
		if strings.Contains(c.Status, "Up") {
			statsCmd := exec.Command("docker", "stats", "--no-stream", "--format", "{{.MemUsage}}|{{.CPUPerc}}", c.ID)
			if statsOut, err := statsCmd.Output(); err == nil {
				statsParts := strings.Split(strings.TrimSpace(string(statsOut)), "|")
				if len(statsParts) >= 2 {
					c.MemUsed = statsParts[0]
					c.CPUUsed = statsParts[1]
				}
			}
		}

		// ▼▼▼ 障害診断 (docker inspect) ▼▼▼
		// 停止(Exited)または死んでいる(Dead)コンテナの詳細を検査
		if strings.Contains(c.Status, "Exited") || strings.Contains(c.Status, "Dead") {
			// ExitCode, OOMKilled, Error を取得
			inspectCmd := exec.Command("docker", "inspect", "--format", "{{.State.ExitCode}}|{{.State.OOMKilled}}|{{.State.Error}}", c.ID)
			if inspectOut, err := inspectCmd.Output(); err == nil {
				// 出力例: "137|true|"
				insParts := strings.Split(strings.TrimSpace(string(inspectOut)), "|")
				if len(insParts) >= 3 {
					// ExitCodeのパース
					fmt.Sscanf(insParts[0], "%d", &c.ExitCode)
					// OOMKilledの判定
					c.OOMKilled = (insParts[1] == "true")
					// エラーメッセージ
					c.Error = insParts[2]
				}
			}
		}
		// ▲▲▲ 障害診断ここまで ▲▲▲

		containers = append(containers, c)
	}

	return &DockerContext{
		IsRunning:  true,
		Count:      len(containers),
		Containers: containers,
	}, nil
}

// CollectProcessContext はプロセスとポート情報を収集します
func CollectProcessContext() (*ProcessContext, error) {
	// 既存monitorのListAllPortsは文字列を返すため、lsofを再実行して構造化する
	// ここでは簡略化のためmonitor.GetListeningPorts (前回提示されたファイルには未実装だが想定)
	// または独自に実装

	ports := getListeningPortsStruct()

	return &ProcessContext{
		NodeProcess:    checkProcessDetail("node"),
		PythonProcess:  checkProcessDetail("python"),
		ListeningPorts: ports,
	}, nil
}

func checkProcessDetail(name string) ProcessDetail {
	cmd := exec.Command("pgrep", "-n", name) // -n: newest
	out, err := cmd.Output()
	if err != nil {
		return ProcessDetail{Detected: false}
	}
	pid := strings.TrimSpace(string(out))

	// コマンドライン取得
	cmdLine := ""
	if cmdCmd := exec.Command("ps", "-p", pid, "-o", "command="); cmdCmd.Run() == nil {
		if out, err := cmdCmd.Output(); err == nil {
			cmdLine = strings.TrimSpace(string(out))
		}
	}

	// バージョン取得 (node -v, python --version)
	version := ""
	// 実行中のプロセスからバージョンを取るのは難しいので、パスの通っているコマンドで代用
	if verCmd := exec.Command(name, "--version"); verCmd.Run() == nil {
		// nodeは -v, pythonは --version など引数が違うため簡易実装
		// 実際はエラーハンドリングが必要
	}

	return ProcessDetail{
		Detected: true,
		Pid:      pid,
		Command:  cmdLine,
		Version:  version,
	}
}

func getListeningPortsStruct() []PortInfo {
	// lsof -i -P -n
	cmd := exec.Command("lsof", "-i", "-P", "-n")
	output, err := cmd.Output()
	if err != nil {
		return []PortInfo{}
	}

	var ports []PortInfo
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if !strings.Contains(line, "LISTEN") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 9 {
			// command, pid, ..., name(port)
			portStr := fields[8]
			if idx := strings.LastIndex(portStr, ":"); idx != -1 {
				portStr = portStr[idx+1:]
			}

			ports = append(ports, PortInfo{
				Process: fields[0],
				Pid:     fields[1],
				Port:    portStr,
			})
		}
	}
	return ports
}

// CollectDatabaseContext はDB状態を収集します
func CollectDatabaseContext() (*DatabaseContext, error) {
	return &DatabaseContext{
		Postgres: checkDBStatus("postgres", "5432"),
		MySQL:    checkDBStatus("mysqld", "3306"),
		Redis:    checkDBStatus("redis-server", "6379"),
	}, nil
}

func checkDBStatus(procName, defaultPort string) DBStatus {
	// プロセスチェック
	cmd := exec.Command("pgrep", procName)
	if err := cmd.Run(); err != nil {
		return DBStatus{IsRunning: false}
	}

	// ポートチェック (lsofで簡易確認)
	// 本来はSQLで接続確認すべきだが、パスワード等の問題があるためポートリッスン確認に留める
	return DBStatus{
		IsRunning: true,
		Port:      defaultPort, // 簡易実装: 実際はlsofから特定すべき
		Message:   "Process is running",
	}
}

// CollectProjectContext はカレントディレクトリ周辺のプロジェクト情報を収集します
func CollectProjectContext() (*ProjectContext, error) {
	cwd, _ := os.Getwd()
	var projects []ProjectInfo

	// カレントディレクトリのチェック
	if info, ok := analyzeProject(cwd); ok {
		projects = append(projects, info)
	}

	// 親ディレクトリやサブディレクトリも探索するロジックがあればここに追加

	return &ProjectContext{
		CollectedAt: time.Now(),
		CurrentDir:  cwd,
		Projects:    projects,
	}, nil
}

func analyzeProject(dir string) (ProjectInfo, bool) {
	var pType string
	var deps []string

	// Node.js
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
		pType = "Node.js"
		// package.jsonの読み込みとdependencies抽出
		if content, err := os.ReadFile(filepath.Join(dir, "package.json")); err == nil {
			var pkg struct {
				Name         string            `json:"name"`
				Dependencies map[string]string `json:"dependencies"`
			}
			if json.Unmarshal(content, &pkg) == nil {
				for k := range pkg.Dependencies {
					deps = append(deps, k)
				}
				// 依存が多い場合はTop10程度に絞る
				if len(deps) > 10 {
					deps = deps[:10]
				}
				return ProjectInfo{Name: pkg.Name, Type: pType, Path: dir, Dependencies: deps}, true
			}
		}
	}

	// Go
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
		pType = "Go"
		// go.modの解析 (簡易: require行を抽出)
		if content, err := os.ReadFile(filepath.Join(dir, "go.mod")); err == nil {
			lines := strings.Split(string(content), "\n")
			for _, line := range lines {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					// モジュール名
				} else if !strings.HasPrefix(line, "//") && strings.Contains(line, " ") {
					// 簡易的に依存とみなす
					parts := strings.Fields(line)
					if len(parts) >= 2 && !strings.Contains(parts[0], "require") && !strings.Contains(parts[0], ")") {
						deps = append(deps, parts[0])
					}
				}
			}
			if len(deps) > 10 {
				deps = deps[:10]
			}
			return ProjectInfo{Name: filepath.Base(dir), Type: pType, Path: dir, Dependencies: deps}, true
		}
	}

	// Python
	if _, err := os.Stat(filepath.Join(dir, "requirements.txt")); err == nil {
		pType = "Python"
		return ProjectInfo{Name: filepath.Base(dir), Type: pType, Path: dir, Dependencies: []string{"requirements.txt detected"}}, true
	}

	return ProjectInfo{}, false
}

// CollectAllContext は全ての情報を一括収集します
func CollectAllContext() (*FullContext, error) {
	// エラーはログに出すなどして、部分的に失敗してもnil以外を返す設計が望ましい
	sys, _ := CollectSystemContext()
	docker, _ := CollectDockerContext()
	proc, _ := CollectProcessContext()
	db, _ := CollectDatabaseContext()
	proj, _ := CollectProjectContext()

	return &FullContext{
		System:   sys,
		Docker:   docker,
		Process:  proc,
		Database: db,
		Project:  proj,
	}, nil
}

// ==========================================
// 3. Formatting Logic
// ==========================================

// FormatAsJSON converts context to JSON string
func FormatAsJSON(ctx interface{}) (string, error) {
	bytes, err := json.MarshalIndent(ctx, "", "  ")
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// FormatAsMarkdown converts context to Markdown for LLM prompt
// Interface型を受け取るように修正
func FormatAsMarkdown(ctx interface{}) (string, error) {
	// 型アサーション
	c, ok := ctx.(*FullContext)
	if !ok {
		return "", fmt.Errorf("invalid context type")
	}

	var sb strings.Builder

	sb.WriteString("# System Environment Report\n\n")
	sb.WriteString(fmt.Sprintf("**Generated At:** %s\n", c.Project.CollectedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("**Current Dir:** %s\n\n", c.Project.CurrentDir))

	// 1. System
	sb.WriteString("## 1. System Resources\n")
	sb.WriteString(fmt.Sprintf("- **CPU Usage:** %.1f%%\n", c.System.CPUUsage))
	sb.WriteString(fmt.Sprintf("- **Memory:** %dMB / %dMB (%.1f%%)\n", c.System.MemoryUsed, c.System.MemoryTotal, c.System.MemoryPerc))
	sb.WriteString(fmt.Sprintf("- **Disk Usage:** %.1f%%\n", c.System.DiskUsage))
	sb.WriteString("\n**Top Processes:**\n")
	for _, p := range c.System.TopProcesses {
		sb.WriteString(fmt.Sprintf("- `%s` (PID: %s): CPU %.1f%%, Mem %dMB\n", p.Name, p.PID, p.CPU, p.Memory))
	}
	sb.WriteString("\n")

	// 2. Project
	sb.WriteString("## 2. Project Context\n")
	if len(c.Project.Projects) == 0 {
		sb.WriteString("No project files detected in current directory.\n")
	} else {
		for _, p := range c.Project.Projects {
			sb.WriteString(fmt.Sprintf("### %s (%s)\n", p.Name, p.Type))
			sb.WriteString(fmt.Sprintf("- Path: `%s`\n", p.Path))
			sb.WriteString("- Dependencies:\n")
			for _, d := range p.Dependencies {
				sb.WriteString(fmt.Sprintf("  - %s\n", d))
			}
		}
	}
	sb.WriteString("\n")

	// 3. Docker
	sb.WriteString("## 3. Docker Status\n")
	if !c.Docker.IsRunning {
		sb.WriteString("Docker is not running.\n")
	} else {
		sb.WriteString(fmt.Sprintf("Containers: %d\n", c.Docker.Count))
		if len(c.Docker.Containers) > 0 {
			// ▼ テーブルヘッダーに Info 列を追加
			sb.WriteString("| ID | Image | Status | Ports | CPU | Mem | Info |\n")
			sb.WriteString("|---|---|---|---|---|---|---|\n")
			for _, cnt := range c.Docker.Containers {
				// ▼ 診断情報の整形
				info := ""
				if cnt.OOMKilled {
					info = "⚠️ **OOM KILLED**"
				} else if cnt.ExitCode != 0 {
					info = fmt.Sprintf("Exit: %d", cnt.ExitCode)
					if cnt.Error != "" {
						info += fmt.Sprintf(" (%s)", cnt.Error)
					}
				}

				// IDが短い場合の対策
				idShort := cnt.ID
				if len(idShort) > 4 {
					idShort = idShort[:4]
				}

				sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s | %s |\n",
					idShort, cnt.Image, cnt.Status, cnt.Ports, cnt.CPUUsed, cnt.MemUsed, info))
			}
		}
	}
	sb.WriteString("\n")

	// 4. Databases
	sb.WriteString("## 4. Databases\n")
	formatDB := func(name string, s DBStatus) {
		status := "STOPPED"
		if s.IsRunning {
			status = fmt.Sprintf("RUNNING (Port: %s)", s.Port)
		}
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", name, status))
	}
	formatDB("PostgreSQL", c.Database.Postgres)
	formatDB("MySQL", c.Database.MySQL)
	formatDB("Redis", c.Database.Redis)

	return sb.String(), nil
}
