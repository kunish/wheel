package relay

import "github.com/kunish/wheel/apps/worker/internal/types"

// MatchGroup finds a group whose name exactly matches the requested model.
func MatchGroup(model string, groups []types.Group) *types.Group {
	for i := range groups {
		if groups[i].Name == model {
			return &groups[i]
		}
	}
	return nil
}
