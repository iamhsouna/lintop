//go:build linux

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Thunderbolt types are only needed so the shared UI/headless code compiles.
// Linux Thunderbolt support is intentionally minimal.
type ThunderboltInfo struct {
	Items []ThunderboltBus `json:"SPThunderboltDataType"`
}

type StorageItem struct {
	Name          string `json:"_name"`
	MountPoint    string `json:"mount_point"`
	PhysicalDrive struct {
		DeviceName string `json:"device_name"`
		IsInternal string `json:"is_internal_disk"`
		Protocol   string `json:"protocol"`
		MediumType string `json:"medium_type"`
	} `json:"physical_drive"`
}

type ThunderboltBus struct {
	Name          string                 `json:"_name"`
	Vendor        string                 `json:"vendor_name_key"`
	DomainUUID    string                 `json:"domain_uuid_key"`
	SwitchUID     string                 `json:"switch_uid_key"`
	Receptacle    *ThunderboltReceptacle `json:"receptacle_1_tag"`
	ConnectedDevs []ThunderboltDevice    `json:"_items"`
	NetworkStats  *ThunderboltNetStats   `json:"network_stats,omitempty"`
}

type ThunderboltReceptacle struct {
	Status       string `json:"receptacle_status_key"`
	CurrentSpeed string `json:"current_speed_key"`
	ReceptacleID string `json:"receptacle_id_key"`
}

type ThunderboltDevice struct {
	Name       string `json:"_name"`
	Vendor     string `json:"vendor_name_key"`
	VendorID   string `json:"vendor_id_key"`
	Mode       string `json:"mode_key"`
	DeviceName string `json:"device_name_key"`
	SwitchUID  string `json:"switch_uid_key"`
	DeviceID   string `json:"device_id_key"`
	DomainUUID string `json:"domain_uuid_key"`
}

type USBInfo struct {
	Items []USBBus `json:"SPUSBDataType"`
}

type USBBus struct {
	Name       string      `json:"_name"`
	HostCtrl   string      `json:"host_controller"`
	PCIDev     string      `json:"pci_device"`
	PCIVendor  string      `json:"pci_vendor"`
	USBDevices []USBDevice `json:"_items"`
}

type USBDevice struct {
	Name         string `json:"_name"`
	Manufacturer string `json:"manufacturer"`
	ProductID    string `json:"product_id"`
	VendorID     string `json:"vendor_id"`
	Speed        string `json:"device_speed"`
	LocationID   string `json:"location_id"`
}

type USBDeviceWithPort struct {
	Device     ThunderboltDeviceOutput
	PortNumber string
}

func GetUSBDevicesFromItems(items []StorageItem) []USBDeviceWithPort { return nil }

type ThunderboltOutput struct {
	Buses []ThunderboltBusOutput `json:"buses"`
}

type ThunderboltBusOutput struct {
	Name         string                    `json:"name"`
	Status       string                    `json:"status"`
	Icon         string                    `json:"icon"`
	Speed        string                    `json:"speed,omitempty"`
	DomainUUID   string                    `json:"domain_uuid,omitempty"`
	SwitchUID    string                    `json:"switch_uid,omitempty"`
	ReceptacleID string                    `json:"receptacle_id,omitempty"`
	Devices      []ThunderboltDeviceOutput `json:"devices,omitempty"`
	NetworkStats *ThunderboltNetStats      `json:"network_stats,omitempty"`
	RDMADevice   *RDMADevice               `json:"rdma_device,omitempty"`
}

type ThunderboltDeviceOutput struct {
	Name       string `json:"name"`
	Vendor     string `json:"vendor,omitempty"`
	VendorID   string `json:"vendor_id,omitempty"`
	Mode       string `json:"mode,omitempty"`
	SwitchUID  string `json:"switch_uid,omitempty"`
	DeviceID   string `json:"device_id,omitempty"`
	DomainUUID string `json:"domain_uuid,omitempty"`
	Info       string `json:"info_string,omitempty"`
}

// GetFormattedThunderboltInfo enumerates connected Thunderbolt devices from
// sysfs. Most Linux desktops have no Thunderbolt controller, in which case an
// empty result is returned.
func GetFormattedThunderboltInfo() (*ThunderboltOutput, error) {
	output := &ThunderboltOutput{}

	entries, _ := filepath.Glob("/sys/bus/thunderbolt/devices/*")
	for _, e := range entries {
		name := filepath.Base(e)
		if strings.HasPrefix(name, "domain") {
			continue
		}
		devName := readTrim(filepath.Join(e, "device_name"))
		if devName == "" {
			continue
		}
		vendor := readTrim(filepath.Join(e, "vendor_name"))
		dev := ThunderboltDeviceOutput{Name: devName, Vendor: vendor}
		if vendor != "" {
			dev.Info = vendor
		}
		output.Buses = append(output.Buses, ThunderboltBusOutput{
			Name:    name,
			Status:  "Active",
			Icon:    "ϟ",
			Devices: []ThunderboltDeviceOutput{dev},
		})
	}
	return output, nil
}

func GetThunderboltDescription() string {
	formatted, err := GetFormattedThunderboltInfo()
	if err != nil {
		return "Error loading Thunderbolt info."
	}
	if len(formatted.Buses) == 0 {
		return "No Thunderbolt controllers found."
	}
	var sb strings.Builder
	for _, bus := range formatted.Buses {
		fmt.Fprintf(&sb, "%s %s (%s)\n", bus.Icon, bus.Name, bus.Status)
		for i, dev := range bus.Devices {
			prefix := "  ├─"
			if i == len(bus.Devices)-1 {
				prefix = "  └─"
			}
			if dev.Info != "" {
				fmt.Fprintf(&sb, "%s %s (%s)\n", prefix, dev.Name, dev.Info)
			} else {
				fmt.Fprintf(&sb, "%s %s\n", prefix, dev.Name)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

var _ = os.Stat
