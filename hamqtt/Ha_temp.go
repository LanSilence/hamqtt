package hamqtt

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Win32_TemperatureProbe struct {
	CurrentTemperature uint32
	Description        string
}

func getCPUTemperatureLinux() (float32, error) {
	files, err := filepath.Glob("/sys/class/thermal/thermal_zone*/temp")
	if err != nil {
		return 0, err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		// 读取温度值(通常是摄氏度乘以1000)
		tempStr := strings.TrimSpace(string(data))
		temp, err := strconv.Atoi(tempStr)
		if err != nil {
			continue
		}

		// 检查是否是CPU温度(可选)
		typeData, err := os.ReadFile(strings.Replace(file, "temp", "type", 1))
		if err == nil && strings.Contains(strings.ToLower(string(typeData)), "cpu") {
			return float32(temp) / 1000, nil
		}
	}

	return 0, fmt.Errorf("CPU temperature not found")
}

func getDeviceTemperature() float64 {
	// 这里可以添加获取设备温度的逻辑
	if getOSType() == "windows" {
		// temp, err := getCPUTemperatureWindows()
		// if err != nil {
		// 	fmt.Println("Error getting CPU temperature:", err)
		// 	return -0.001 // 返回默认值
		// }
		return -0.001
	} else if getOSType() == "linux" {
		temp, err := getCPUTemperatureLinux()
		if err != nil {
			fmt.Println("Error getting CPU temperature:", err)
			return -0.001 // 返回默认值
		}
		return float64(temp)

	}
	return 25.0
}
