package system

import "strconv"

func trimFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 2, 64)
}
