package model

import "time"

// KnowledgeSearchEvent 记录一次知识库搜索质量统计事件。
type KnowledgeSearchEvent struct {
	ID        string    `json:"id"`
	Query     string    `json:"query"`
	HitCount  int       `json:"hit_count"`
	TopScore  float64   `json:"top_score"`
	Engine    string    `json:"engine"`
	CreatedAt time.Time `json:"created_at"`
}

// KnowledgeSearchBadcase 记录用户标记的不相关搜索结果。
type KnowledgeSearchBadcase struct {
	ID        string    `json:"id"`
	Query     string    `json:"query"`
	DocID     string    `json:"doc_id"`
	Reason    string    `json:"reason"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

// KnowledgeSearchStats 聚合搜索质量统计。
type KnowledgeSearchStats struct {
	SearchCount   int             `json:"search_count"`
	HitRate       float64         `json:"hit_rate"`
	AverageScore  float64         `json:"average_score"`
	TopQueries    []TopQueryStat  `json:"top_queries"`
	BadcaseCount  int             `json:"badcase_count"`
	RecentBadcase []SearchBadcase `json:"recent_badcases"`
}

type TopQueryStat struct {
	Query string `json:"query"`
	Count int    `json:"count"`
}

type SearchBadcase struct {
	Query     string    `json:"query"`
	DocID     string    `json:"doc_id"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
