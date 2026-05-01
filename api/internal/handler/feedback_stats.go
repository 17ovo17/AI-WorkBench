package handler

import (
	"net/http"
	"strconv"

	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

// categoryFeedbackStats 按根因分类聚合的反馈评分统计。
type categoryFeedbackStats struct {
	Category string  `json:"category"`
	Total    int     `json:"total"`
	AvgScore float64 `json:"avg_score"`
}

// FeedbackStats GET /api/v1/diagnosis/feedback/stats
func FeedbackStats(c *gin.Context) {
	allFeedbacks, total := store.ListAllFeedbacks(1, 1000)
	ratingScores := map[string]float64{
		"accurate":   1.0,
		"partial":    0.5,
		"inaccurate": 0.0,
	}

	// 按 diagnosis_id 关联 case 的 root_cause_category 聚合
	catMap := map[string]*categoryFeedbackStats{}
	for _, f := range allFeedbacks {
		category := "unknown"
		if c, ok := store.GetCaseByDiagnosisID(f.DiagnosisID); ok {
			category = c.RootCauseCategory
		}
		stat, exists := catMap[category]
		if !exists {
			stat = &categoryFeedbackStats{Category: category}
			catMap[category] = stat
		}
		stat.Total++
		stat.AvgScore += ratingScores[f.Rating]
	}

	items := make([]categoryFeedbackStats, 0, len(catMap))
	for _, stat := range catMap {
		if stat.Total > 0 {
			stat.AvgScore = stat.AvgScore / float64(stat.Total)
		}
		items = append(items, *stat)
	}

	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"total": total,
	})
}

// ListVerifications GET /api/v1/diagnosis/verifications
func ListVerifications(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	items := store.ListVerifications(limit)
	c.JSON(http.StatusOK, gin.H{"items": items, "total": len(items)})
}
