package system

type UsageMetric struct {
	Used       uint64  `json:"used"`
	Total      uint64  `json:"total"`
	Percent    float64 `json:"percent"`
	UsedText   string  `json:"usedText"`
	TotalText  string  `json:"totalText"`
	DetailText string  `json:"detailText"`
}

type Metrics struct {
	LoadAverage float64     `json:"loadAverage"`
	CPU         UsageMetric `json:"cpu"`
	Memory      UsageMetric `json:"memory"`
	Disk        UsageMetric `json:"disk"`
}

func percent(used, total uint64) float64 {
	if total == 0 {
		return 0
	}
	return float64(used) * 100 / float64(total)
}

func bytesText(value uint64) string {
	const unit = 1024
	if value < unit {
		return formatFloat(float64(value), "B")
	}
	div, exp := uint64(unit), 0
	for n := value / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return formatFloat(float64(value)/float64(div), []string{"KB", "MB", "GB", "TB", "PB"}[exp])
}

func formatFloat(value float64, unit string) string {
	return trimFloat(value) + " " + unit
}
