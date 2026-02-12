package relay

import "time"

func currentUnixSec() int64 {
	return time.Now().Unix()
}
