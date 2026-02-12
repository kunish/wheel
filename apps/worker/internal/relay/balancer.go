package relay

import (
	"math/rand"
	"sort"
	"sync"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// rrCounters tracks round-robin position per group (goroutine-safe).
var rrCounters sync.Map

// SelectChannelOrder returns group items ordered according to the load-balancing mode.
func SelectChannelOrder(mode types.GroupMode, items []types.GroupItem, groupID int) []types.GroupItem {
	if len(items) == 0 {
		return items
	}

	switch mode {
	case types.GroupModeRoundRobin:
		return roundRobin(items, groupID)
	case types.GroupModeRandom:
		return randomOrder(items)
	case types.GroupModeFailover:
		return failoverOrder(items)
	case types.GroupModeWeighted:
		return weightedOrder(items)
	default:
		return items
	}
}

func roundRobin(items []types.GroupItem, groupID int) []types.GroupItem {
	val, _ := rrCounters.LoadOrStore(groupID, 0)
	idx := val.(int) % len(items)
	rrCounters.Store(groupID, idx+1)

	result := make([]types.GroupItem, 0, len(items))
	result = append(result, items[idx:]...)
	result = append(result, items[:idx]...)
	return result
}

func randomOrder(items []types.GroupItem) []types.GroupItem {
	shuffled := make([]types.GroupItem, len(items))
	copy(shuffled, items)
	// Fisher-Yates shuffle
	for i := len(shuffled) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	return shuffled
}

func failoverOrder(items []types.GroupItem) []types.GroupItem {
	sorted := make([]types.GroupItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	return sorted
}

func weightedOrder(items []types.GroupItem) []types.GroupItem {
	totalWeight := 0
	for _, it := range items {
		totalWeight += it.Weight
	}
	if totalWeight == 0 {
		return items
	}

	r := rand.Float64() * float64(totalWeight)
	selected := 0
	for i, it := range items {
		r -= float64(it.Weight)
		if r <= 0 {
			selected = i
			break
		}
	}

	result := make([]types.GroupItem, 0, len(items))
	result = append(result, items[selected])
	for i, it := range items {
		if i != selected {
			result = append(result, it)
		}
	}
	return result
}
