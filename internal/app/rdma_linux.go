//go:build linux

package app

import (
	"os/exec"
	"strings"
	"sync"
	"time"
)

// RDMADevice holds information about a single RDMA device
type RDMADevice struct {
	Name      string `json:"name" yaml:"name" xml:"Name"`
	NodeGUID  string `json:"node_guid" yaml:"node_guid" xml:"NodeGUID"`
	Transport string `json:"transport" yaml:"transport" xml:"Transport"`
	PortState string `json:"port_state" yaml:"port_state" xml:"PortState"`
	ActiveMTU int    `json:"active_mtu" yaml:"active_mtu" xml:"ActiveMTU"`
	LinkLayer string `json:"link_layer" yaml:"link_layer" xml:"LinkLayer"`
	Interface string `json:"interface,omitempty" yaml:"interface,omitempty" xml:"Interface,omitempty"`
}

// RDMAStatus holds RDMA availability information
type RDMAStatus struct {
	Available bool         `json:"available" yaml:"available" xml:"Available"`
	Status    string       `json:"status" yaml:"status" xml:"Status"`
	Devices   []RDMADevice `json:"devices,omitempty" yaml:"devices,omitempty" xml:"Devices,omitempty"`
}

var (
	rdmaMutex      sync.Mutex
	lastRDMAStatus RDMAStatus
	lastRDMACheck  time.Time
	rdmaCacheTTL   = 10 * time.Second
)

// CheckRDMAAvailable detects RDMA devices via ibv_devinfo on Linux.
func CheckRDMAAvailable() RDMAStatus {
	rdmaMutex.Lock()
	defer rdmaMutex.Unlock()

	if time.Since(lastRDMACheck) < rdmaCacheTTL {
		return lastRDMAStatus
	}

	status := RDMAStatus{Available: false, Status: "RDMA not available"}

	if _, err := exec.LookPath("ibv_devinfo"); err != nil {
		lastRDMAStatus = status
		lastRDMACheck = time.Now()
		return status
	}

	devices := GetRDMADevices()
	if len(devices) > 0 {
		status.Available = true
		status.Status = "Enabled"
		status.Devices = devices
	} else {
		status.Status = "RDMA not available (no devices)"
	}

	lastRDMAStatus = status
	lastRDMACheck = time.Now()
	return status
}

// GetRDMADevices parses ibv_devinfo output.
func GetRDMADevices() []RDMADevice {
	out, err := exec.Command("ibv_devinfo").Output()
	if err != nil {
		return nil
	}
	var devices []RDMADevice
	var current *RDMADevice
	for _, line := range strings.Split(string(out), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "hca_id:") {
			if current != nil {
				devices = append(devices, *current)
			}
			current = &RDMADevice{Name: strings.TrimSpace(strings.TrimPrefix(trimmed, "hca_id:"))}
			continue
		}
		if current == nil {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "transport:"):
			current.Transport = strings.TrimSpace(strings.TrimPrefix(trimmed, "transport:"))
		case strings.HasPrefix(trimmed, "state:"):
			current.PortState = strings.TrimSpace(strings.TrimPrefix(trimmed, "state:"))
		case strings.HasPrefix(trimmed, "link_layer:"):
			current.LinkLayer = strings.TrimSpace(strings.TrimPrefix(trimmed, "link_layer:"))
		case strings.HasPrefix(trimmed, "active_mtu:"):
			current.ActiveMTU = atoiSafe(strings.TrimSpace(strings.TrimPrefix(trimmed, "active_mtu:")))
		case strings.HasPrefix(trimmed, "node_guid:"):
			current.NodeGUID = strings.TrimSpace(strings.TrimPrefix(trimmed, "node_guid:"))
		}
	}
	if current != nil {
		devices = append(devices, *current)
	}
	return devices
}
