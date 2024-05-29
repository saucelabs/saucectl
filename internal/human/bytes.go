package human

import (
	"fmt"
	"math"
)

func Bytes(b int64) string {
	sizes := []string{"B", "kB", "MB", "GB"}
	e := math.Floor(math.Log(float64(b)) / math.Log(1000))
	suffix := sizes[int(math.Min(e, float64(len(sizes)-1)))]
	val := float64(b) / math.Pow(1000, e)
	return fmt.Sprintf("%.0f %s", val, suffix)
}
