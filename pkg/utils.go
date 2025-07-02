package pkg

import "runtime"

// GetOSType 返回操作系统类型
func GetOSType() string {
	return runtime.GOOS
}
