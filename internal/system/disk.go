package system

import (
	"fmt"

	"github.com/shirou/gopsutil/disk"
)

type DiskInfo struct {
	Mountpoint  string  `json:"mountpoint"`
	Total       uint64  `json:"total"`
	Free        uint64  `json:"free"`
	Used        uint64  `json:"used"`
	UsedPercent float64 `json:"used_percent"`
}

// GetAllDisksInfo 获取所有磁盘分区信息
func GetAllDisksInfo() ([]DiskInfo, error) {
	partitions, err := disk.Partitions(false)
	if err != nil {
		return nil, err
	}
	var disks []DiskInfo
	for _, p := range partitions {
		usage, err := disk.Usage(p.Mountpoint)
		if err != nil {
			// 某些分区可能无法访问，跳过
			continue
		}
		disks = append(disks, DiskInfo{
			Mountpoint:  p.Mountpoint,
			Total:       usage.Total,
			Free:        usage.Free,
			Used:        usage.Used,
			UsedPercent: usage.UsedPercent,
		})
	}
	return disks, nil
}

// 打印所有磁盘信息（调试用）
func PrintAllDisksInfo() {
	disks, err := GetAllDisksInfo()
	if err != nil {
		fmt.Println("获取磁盘信息失败:", err)
		return
	}
	for _, d := range disks {
		if d.Mountpoint == "/" {
			fmt.Printf("总大小: %d 字节\n", d.Total)
			fmt.Printf("可用大小: %d 字节\n", d.Free)
			fmt.Printf("已用大小: %d 字节\n", d.Used)
			fmt.Printf("使用率: %.2f%%\n", d.UsedPercent)
		}
		break
	}
}
