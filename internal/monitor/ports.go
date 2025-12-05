package monitor

import (
	"os/exec"
	"strings"
)

// PortInfo holds information about a listening port
type PortInfo struct {
	Port    string
	Process string
	PID     string
}

// GetListeningPorts returns all listening ports
func GetListeningPorts() []PortInfo {
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
		if len(fields) < 9 {
			continue
		}
		
		portInfo := fields[8]
		if strings.Contains(portInfo, ":") {
			parts := strings.Split(portInfo, ":")
			port := parts[len(parts)-1]
			
			ports = append(ports, PortInfo{
				Port:    port,
				Process: fields[0],
				PID:     fields[1],
			})
		}
	}
	
	return ports
}

// ListAllPorts displays all listening ports
func ListAllPorts() string {
	ports := GetListeningPorts()
	
	if len(ports) == 0 {
		return "ポート: 検出なし"
	}
	
	result := "使用中のポート:\n"
	for _, p := range ports {
		result += "  :" + p.Port + " - " + p.Process + "\n"
	}
	
	return result
}
