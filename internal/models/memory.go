package models

type Memory struct {
	ID         int64     `json:"-"`
	UUID       string    `json:"id"`
	Role       string    `json:"role"`
	Content    string    `json:"content"`
	Timestamp  string    `json:"timestamp"`
	UserMsg    string    `json:"user_msg,omitempty"`
	AssistMsg  string    `json:"assist_msg,omitempty"`
	Type       string    `json:"type"`
	Embedding  []float32 `json:"-"`
}

type MemoryResult struct {
	ID            string  `json:"id"`
	Content       string  `json:"content"`
	Similarity    float32 `json:"similarity"`
	Timestamp     string  `json:"timestamp"`
	UserMsg       string  `json:"user_msg,omitempty"`
	AssistMsg     string  `json:"assist_msg,omitempty"`
	MatchType     string  `json:"match_type,omitempty"`    // "vector", "fts", "hybrid"
	Importance    int     `json:"importance,omitempty"`    // 1–5; default 3
	Source        string  `json:"source,omitempty"`        // "conversation" | "explicit"
	Tags          string  `json:"tags,omitempty"`          // comma-separated
	SessionID     string  `json:"session_id,omitempty"`
	RetrieveCount int     `json:"retrieve_count,omitempty"`
}

type MemoryFileInfo struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	SizeKB   int64  `json:"size_kb"`
	Modified string `json:"modified"`
}

type SearchParams struct {
	Query         string
	TopK          int
	MinSimilarity float32
}

type MemoryStats struct {
	Count          int            `json:"count"`
	VecCount       int            `json:"vec_count"`
	Dimension      int            `json:"dimension"`
	StorageBytes   int64          `json:"storage_bytes"`
	ExplicitCount  int            `json:"explicit_count"`
	AddedThisWeek  int            `json:"added_this_week"`
	PendingDeletion int           `json:"pending_deletion"`
	TopRetrieved   []MemoryResult `json:"top_retrieved"`
}
