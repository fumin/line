package util

import "time"

var (
	TaipeiTZ = time.FixedZone("Asia/Taipei", int((8 * time.Hour).Seconds()))
)
