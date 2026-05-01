package handler

import (
	"net/http"
	"time"

	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
)

func DashboardSummary(c *gin.Context) {
	agents := store.ListAgents()
	onlineCount := 0
	for _, a := range agents {
		if store.HasOnlineAgent(a.IP) {
			onlineCount++
		}
	}

	alertStats := buildAlertStats()

	records := store.ListRecords()
	recentDiagnoses := records
	if len(recentDiagnoses) > 5 {
		recentDiagnoses = recentDiagnoses[:5]
	}

	businesses := store.ListTopologyBusinesses()

	c.JSON(http.StatusOK, gin.H{
		"agents":           gin.H{"total": len(agents), "online": onlineCount},
		"alerts":           alertStats,
		"recent_diagnoses": recentDiagnoses,
		"businesses":       len(businesses),
		"profiles":         len(store.ListUserProfiles()),
		"workflow_stats":   buildWorkflowStats(),
		"knowledge_stats":  buildKnowledgeStats(),
		"feedback_stats":   buildFeedbackStats(),
		"recent_changes":   store.ListChangeEvents("", time.Time{}, 5),
	})
}

func buildAlertStats() map[string]int {
	allAlerts := store.ListAlerts()
	stats := map[string]int{"total": 0, "critical": 0, "warning": 0, "info": 0, "firing": 0}
	for _, a := range allAlerts {
		if a.DeletedAt != nil {
			continue
		}
		stats["total"]++
		switch a.Severity {
		case "critical":
			stats["critical"]++
		case "warning":
			stats["warning"]++
		default:
			stats["info"]++
		}
		if a.Status == "firing" {
			stats["firing"]++
		}
	}
	return stats
}

func buildWorkflowStats() gin.H {
	wfs := store.ListWorkflows()
	var totalRuns int
	var successCount int
	var totalElapsed int64
	for _, wf := range wfs {
		runs, count := store.ListWorkflowRuns(wf.ID, 1, 200)
		totalRuns += count
		for _, r := range runs {
			if r.Status == "succeeded" {
				successCount++
			}
			totalElapsed += r.ElapsedMs
		}
	}
	successRate := 0.0
	avgElapsed := int64(0)
	if totalRuns > 0 {
		successRate = float64(successCount) / float64(totalRuns) * 100
		avgElapsed = totalElapsed / int64(totalRuns)
	}
	return gin.H{
		"total_runs":     totalRuns,
		"success_rate":   successRate,
		"avg_elapsed_ms": avgElapsed,
	}
}

func buildKnowledgeStats() gin.H {
	_, casesTotal := store.ListCases(1, 1, "", "")
	_, runbooksTotal := store.ListRunbooks("", 1, 1)
	_, docsTotal := store.ListDocuments(1, 1, "", "", "")
	return gin.H{
		"cases":     casesTotal,
		"runbooks":  runbooksTotal,
		"documents": docsTotal,
	}
}

func buildFeedbackStats() gin.H {
	_, total := store.ListAllFeedbacks(1, 1)
	allFeedbacks, _ := store.ListAllFeedbacks(1, 1000)
	ratingScores := map[string]float64{
		"accurate":   1.0,
		"partial":    0.5,
		"inaccurate": 0.0,
	}
	sum := 0.0
	for _, f := range allFeedbacks {
		sum += ratingScores[f.Rating]
	}
	avgRating := 0.0
	if len(allFeedbacks) > 0 {
		avgRating = sum / float64(len(allFeedbacks))
	}
	return gin.H{
		"total":      total,
		"avg_rating": avgRating,
	}
}
