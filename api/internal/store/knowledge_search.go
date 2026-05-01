package store

import (
	"time"

	"ai-workbench-api/internal/model"
)

// AddKnowledgeSearchEvent 记录知识库搜索事件。
func AddKnowledgeSearchEvent(e *model.KnowledgeSearchEvent) {
	if e.ID == "" {
		e.ID = NewID()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now()
	}
	mu.Lock()
	searchEvents = append([]*model.KnowledgeSearchEvent{e}, searchEvents...)
	if len(searchEvents) > 2000 {
		searchEvents = searchEvents[:2000]
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`INSERT INTO knowledge_search_events (id,query,hit_count,top_score,engine,created_at) VALUES (?,?,?,?,?,?)`,
			e.ID, e.Query, e.HitCount, e.TopScore, e.Engine, e.CreatedAt)
	}
}

// AddKnowledgeSearchBadcase 记录用户标记的不相关搜索结果。
func AddKnowledgeSearchBadcase(b *model.KnowledgeSearchBadcase) {
	if b.ID == "" {
		b.ID = NewID()
	}
	if b.CreatedAt.IsZero() {
		b.CreatedAt = time.Now()
	}
	mu.Lock()
	searchBadcases = append([]*model.KnowledgeSearchBadcase{b}, searchBadcases...)
	if len(searchBadcases) > 1000 {
		searchBadcases = searchBadcases[:1000]
	}
	mu.Unlock()
	if mysqlOK {
		_, _ = db.Exec(`INSERT INTO knowledge_search_badcases (id,query,doc_id,reason,created_by,created_at) VALUES (?,?,?,?,?,?)`,
			b.ID, b.Query, b.DocID, b.Reason, b.CreatedBy, b.CreatedAt)
	}
}

// KnowledgeSearchStats 返回搜索质量统计。
func KnowledgeSearchStats(limit int) model.KnowledgeSearchStats {
	if limit <= 0 || limit > 20 {
		limit = 10
	}
	if mysqlOK {
		return knowledgeSearchStatsMySQL(limit)
	}
	return knowledgeSearchStatsMemory(limit)
}

func knowledgeSearchStatsMemory(limit int) model.KnowledgeSearchStats {
	mu.RLock()
	defer mu.RUnlock()
	stats := model.KnowledgeSearchStats{SearchCount: len(searchEvents), BadcaseCount: len(searchBadcases)}
	queryCount := map[string]int{}
	hits, scoreSum := 0, 0.0
	for _, e := range searchEvents {
		if e.HitCount > 0 {
			hits++
		}
		scoreSum += e.TopScore
		queryCount[e.Query]++
	}
	if stats.SearchCount > 0 {
		stats.HitRate = float64(hits) / float64(stats.SearchCount)
		stats.AverageScore = scoreSum / float64(stats.SearchCount)
	}
	stats.TopQueries = topQueryStats(queryCount, limit)
	stats.RecentBadcase = recentBadcases(searchBadcases, limit)
	return stats
}

func knowledgeSearchStatsMySQL(limit int) model.KnowledgeSearchStats {
	stats := model.KnowledgeSearchStats{}
	_ = db.QueryRow(`SELECT COUNT(*),COALESCE(SUM(hit_count>0),0),COALESCE(AVG(top_score),0) FROM knowledge_search_events`).
		Scan(&stats.SearchCount, &stats.HitRate, &stats.AverageScore)
	if stats.SearchCount > 0 {
		stats.HitRate = stats.HitRate / float64(stats.SearchCount)
	}
	_ = db.QueryRow(`SELECT COUNT(*) FROM knowledge_search_badcases`).Scan(&stats.BadcaseCount)
	stats.TopQueries = queryStatsMySQL(limit)
	stats.RecentBadcase = recentBadcasesMySQL(limit)
	return stats
}

func queryStatsMySQL(limit int) []model.TopQueryStat {
	rows, err := db.Query(`SELECT query,COUNT(*) FROM knowledge_search_events GROUP BY query ORDER BY COUNT(*) DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []model.TopQueryStat{}
	for rows.Next() {
		var item model.TopQueryStat
		_ = rows.Scan(&item.Query, &item.Count)
		out = append(out, item)
	}
	return out
}

func recentBadcasesMySQL(limit int) []model.SearchBadcase {
	rows, err := db.Query(`SELECT query,doc_id,COALESCE(reason,''),created_at FROM knowledge_search_badcases ORDER BY created_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()
	out := []model.SearchBadcase{}
	for rows.Next() {
		var item model.SearchBadcase
		_ = rows.Scan(&item.Query, &item.DocID, &item.Reason, &item.CreatedAt)
		out = append(out, item)
	}
	return out
}
