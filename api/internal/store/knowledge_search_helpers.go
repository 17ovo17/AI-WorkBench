package store

import (
	"sort"

	"ai-workbench-api/internal/model"
)

func topQueryStats(counts map[string]int, limit int) []model.TopQueryStat {
	out := make([]model.TopQueryStat, 0, len(counts))
	for query, count := range counts {
		out = append(out, model.TopQueryStat{Query: query, Count: count})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Count > out[j].Count })
	if len(out) > limit {
		return out[:limit]
	}
	return out
}

func recentBadcases(items []*model.KnowledgeSearchBadcase, limit int) []model.SearchBadcase {
	if len(items) < limit {
		limit = len(items)
	}
	out := make([]model.SearchBadcase, 0, limit)
	for i := 0; i < limit; i++ {
		item := items[i]
		out = append(out, model.SearchBadcase{
			Query:     item.Query,
			DocID:     item.DocID,
			Reason:    item.Reason,
			CreatedAt: item.CreatedAt,
		})
	}
	return out
}
