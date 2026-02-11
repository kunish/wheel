package types

// OutboundType represents the upstream provider type.
type OutboundType int

const (
	OutboundOpenAIChat     OutboundType = 0
	OutboundOpenAI         OutboundType = 1
	OutboundAnthropic      OutboundType = 2
	OutboundGemini         OutboundType = 3
	OutboundVolcengine     OutboundType = 4
	OutboundOpenAIEmbedding OutboundType = 5
)

// GroupMode controls how channels are selected within a group.
type GroupMode int

const (
	GroupModeRoundRobin GroupMode = 1
	GroupModeRandom     GroupMode = 2
	GroupModeFailover   GroupMode = 3
	GroupModeWeighted   GroupMode = 4
)

// AutoGroupType controls automatic group creation behavior.
type AutoGroupType int

const (
	AutoGroupNone  AutoGroupType = 0
	AutoGroupFuzzy AutoGroupType = 1
	AutoGroupExact AutoGroupType = 2
)

// AttemptStatus is the result status of a relay attempt.
type AttemptStatus string

const (
	AttemptStatusSuccess      AttemptStatus = "success"
	AttemptStatusFailed       AttemptStatus = "failed"
	AttemptStatusCircuitBreak AttemptStatus = "circuit_break"
	AttemptStatusSkipped      AttemptStatus = "skipped"
)
