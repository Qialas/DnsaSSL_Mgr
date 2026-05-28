//go:build linux

package system

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"
)

func GetMetrics() Metrics {
	load := readLoadAverage()
	cpuPercent := readCPUPercent()
	memory := readMemory()
	disk := readDisk("/")

	return Metrics{
		LoadAverage: load,
		CPU: UsageMetric{
			Percent:    cpuPercent,
			Total:      uint64(runtime.NumCPU()),
			DetailText: strconv.Itoa(runtime.NumCPU()) + " 核心",
		},
		Memory: memory,
		Disk:   disk,
	}
}

func readLoadAverage() float64 {
	raw, err := os.ReadFile("/proc/loadavg")
	if err != nil {
		return 0
	}
	fields := strings.Fields(string(raw))
	if len(fields) == 0 {
		return 0
	}
	value, _ := strconv.ParseFloat(fields[0], 64)
	return value
}

func readCPUPercent() float64 {
	first, ok := readCPUStat()
	if !ok {
		return 0
	}
	time.Sleep(200 * time.Millisecond)
	second, ok := readCPUStat()
	if !ok {
		return 0
	}
	idle := second.idle - first.idle
	total := second.total - first.total
	if total == 0 {
		return 0
	}
	return float64(total-idle) * 100 / float64(total)
}

type cpuStat struct {
	idle  uint64
	total uint64
}

func readCPUStat() (cpuStat, bool) {
	raw, err := os.ReadFile("/proc/stat")
	if err != nil {
		return cpuStat{}, false
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) == 0 {
		return cpuStat{}, false
	}
	fields := strings.Fields(lines[0])
	if len(fields) < 5 || fields[0] != "cpu" {
		return cpuStat{}, false
	}
	var values []uint64
	for _, field := range fields[1:] {
		value, err := strconv.ParseUint(field, 10, 64)
		if err != nil {
			return cpuStat{}, false
		}
		values = append(values, value)
	}
	idle := values[3]
	if len(values) > 4 {
		idle += values[4]
	}
	var total uint64
	for _, value := range values {
		total += value
	}
	return cpuStat{idle: idle, total: total}, true
}

func readMemory() UsageMetric {
	raw, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return UsageMetric{}
	}
	values := map[string]uint64{}
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		value, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			continue
		}
		values[strings.TrimSuffix(fields[0], ":")] = value * 1024
	}
	total := values["MemTotal"]
	available := values["MemAvailable"]
	used := total - available
	return UsageMetric{
		Used:       used,
		Total:      total,
		Percent:    percent(used, total),
		UsedText:   bytesText(used),
		TotalText:  bytesText(total),
		DetailText: bytesText(used) + " / " + bytesText(total),
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
