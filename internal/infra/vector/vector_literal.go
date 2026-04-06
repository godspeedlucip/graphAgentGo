package vector

import (
	"strconv"
	"strings"
)

func ToLiteral(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	parts := make([]string, 0, len(v))
	for _, x := range v {
		parts = append(parts, strconv.FormatFloat(float64(x), 'f', -1, 32))
	}
	return "[" + strings.Join(parts, ",") + "]"
}