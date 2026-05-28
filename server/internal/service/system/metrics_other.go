//go:build !linux

package system

import (
	"runtime"
	"strconv"
	"syscall"
)

func GetMetrics() Metrics {
	return Metrics{
		CPU: UsageMetric{
			Total:      uint64(runtime.NumCPU()),
			DetailText: strconv.Itoa(runtime.NumCPU()) + " 核心",
		},
		Disk: readDisk("/"),
	}
}

func readDisk(path string) UsageMetric {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return UsageMetric{}
	}
	total := stat.Blocks * uint64(stat.Bsize)
	free := stat.Bavail * uint64(stat.Bsize)
	used := total - free
	return UsageMetric{
		Used:       used,
		Total:      total,
		Percent:    percent(used, total),
		UsedText:   bytesText(used),
		TotalText:  bytesText(total),
		DetailText: bytesText(used) + " / " + bytesText(total),
	}
}
