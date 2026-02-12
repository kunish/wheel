package relay

import (
	"sort"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

const rateLimitCooldown = 300 // 300 seconds (5 minutes)

// SelectKey picks the best available key from a channel's key list.
// Prefers enabled keys not in 429 cooldown; among those, picks lowest totalCost.
// If all are rate-limited, falls back to the one with the oldest lastUseTimestamp.
func SelectKey(keys []types.ChannelKey) *types.ChannelKey {
	now := time.Now().Unix()

	var enabled []types.ChannelKey
	for _, k := range keys {
		if k.Enabled {
			enabled = append(enabled, k)
		}
	}
	if len(enabled) == 0 {
		return nil
	}

	// Prefer keys not in 429 cooldown
	var preferred []types.ChannelKey
	for _, k := range enabled {
		if k.StatusCode == 429 && now-k.LastUseTimestamp < rateLimitCooldown {
			continue
		}
		preferred = append(preferred, k)
	}

	if len(preferred) > 0 {
		sort.Slice(preferred, func(i, j int) bool {
			return preferred[i].TotalCost < preferred[j].TotalCost
		})
		return &preferred[0]
	}

	// Fallback: all rate-limited, pick oldest lastUseTimestamp
	sort.Slice(enabled, func(i, j int) bool {
		return enabled[i].LastUseTimestamp < enabled[j].LastUseTimestamp
	})
	return &enabled[0]
}
