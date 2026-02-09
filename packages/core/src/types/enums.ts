export enum OutboundType {
  OpenAIChat = 0,
  OpenAI = 1,
  Anthropic = 2,
  Gemini = 3,
  Volcengine = 4,
  OpenAIEmbedding = 5,
}

export enum GroupMode {
  RoundRobin = 1,
  Random = 2,
  Failover = 3,
  Weighted = 4,
}

export enum AutoGroupType {
  None = 0,
  Fuzzy = 1,
  Exact = 2,
}
