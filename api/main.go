package main

import (
	"ai-workbench-api/internal/embedding"
	"ai-workbench-api/internal/handler"
	"ai-workbench-api/internal/knowledge"
	"ai-workbench-api/internal/middleware"
	"ai-workbench-api/internal/scheduler"
	"ai-workbench-api/internal/store"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

func main() {
	viper.SetConfigFile("config.yaml")
	if err := viper.ReadInConfig(); err != nil {
		logrus.Fatalf("读取配置失败: %v", err)
	}

	os.MkdirAll("../logs", 0755)
	logFile, _ := os.OpenFile("../logs/api.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	logrus.SetOutput(io.MultiWriter(os.Stdout, logFile))
	store.Init()
	embedding.Init()
	go knowledge.LoadExistingDocsToSearchEngine()
	handler.InitAuth()
	scheduler.SetDiagnoseFunc(handler.RunDiagnose)
	scheduler.Start()
	scheduler.StartWorkflowScheduler()
	handler.StartMetricsAutoSync()

	r := gin.Default()
	r.Use(cors.New(corsConfig()))
	r.Use(middleware.RateLimit(100))
	r.GET("/metrics", handler.Metrics)
	adminRequired := requireAdminToken()

	v1 := r.Group("/api/v1")
	{
		v1.GET("/models", handler.Models)
		v1.POST("/chat", handler.Chat)
		v1.GET("/chat/sessions", handler.ListChatSessions)
		v1.POST("/chat/sessions", handler.CreateChatSession)
		v1.GET("/chat/sessions/:id", handler.GetChatSession)
		v1.PUT("/chat/sessions/:id", handler.RenameChatSession)
		v1.DELETE("/chat/sessions/:id", handler.DeleteChatSession)
		v1.POST("/agent/llm/chat", handler.AgentLLMChat)
		v1.POST("/aiops/sessions", handler.AIOpsCreateSession)
		v1.POST("/aiops/sessions/:id/messages", handler.AIOpsPostMessage)
		v1.GET("/aiops/sessions/:id/messages", handler.AIOpsGetMessages)
		v1.GET("/aiops/ws/sessions/:id", handler.AIOpsSessionWS)
		v1.POST("/aiops/sessions/:id/actions/execute", adminRequired, handler.AIOpsExecuteAction)
		v1.POST("/aiops/inspections", adminRequired, handler.AIOpsCreateInspection)
		v1.GET("/aiops/inspections/:id/progress", handler.AIOpsInspectionProgress)
		v1.GET("/aiops/inspections/:id/report", handler.AIOpsInspectionReport)
		v1.POST("/aiops/data/prometheus/query", adminRequired, handler.AIOpsPrometheusQuery)
		v1.POST("/aiops/data/prometheus/query_range", adminRequired, handler.AIOpsPrometheusQueryRange)
		v1.POST("/aiops/data/catpaw/query", adminRequired, handler.AIOpsCatpawQuery)
		v1.POST("/aiops/topology/generate", adminRequired, handler.AIOpsTopologyGenerate)

		v1.POST("/diagnose", adminRequired, handler.StartDiagnose)
		v1.GET("/diagnose", handler.ListDiagnose)
		v1.GET("/diagnose/compare", handler.CompareDiagnoses)
		v1.DELETE("/diagnose", adminRequired, handler.CleanupDiagnose)
		v1.DELETE("/diagnose/:id", adminRequired, handler.DeleteDiagnose)

		v1.POST("/catpaw/heartbeat", handler.CatpawHeartbeat)
		v1.POST("/catpaw/report", handler.CatpawReport)
		v1.GET("/catpaw/agents", handler.ListAgents)
		v1.DELETE("/catpaw/agents/:ip", adminRequired, handler.DeleteAgent)

		v1.POST("/alert/webhook", handler.AlertWebhook)
		v1.POST("/alert/catpaw", handler.CatpawAlert)
		v1.GET("/alerts", handler.ListAlerts)
		v1.PUT("/alerts/:id/resolve", adminRequired, handler.ResolveAlert)
		v1.PUT("/alerts/:id/:action", adminRequired, handler.AlertAction)
		v1.DELETE("/alerts/:id", adminRequired, handler.DeleteAlert)

		v1.POST("/remote/exec", adminRequired, handler.RemoteExec)
		v1.POST("/remote/check-port", adminRequired, handler.CheckRemotePort)
		v1.POST("/remote/install-catpaw", adminRequired, handler.InstallCatpaw)
		v1.POST("/remote/uninstall-catpaw", adminRequired, handler.UninstallCatpaw)
		v1.POST("/remote/install-cmd", adminRequired, handler.GenerateInstallCmd)

		v1.GET("/catpaw/chat-ws", handler.CatpawChat)

		v1.GET("/platform/ip", handler.GetPlatformIP)
		v1.GET("/prometheus/instances", handler.GetPrometheusInstances)
		v1.GET("/prometheus/hosts", handler.PrometheusHosts)
		v1.GET("/prometheus/metrics", handler.PrometheusMetrics)
		v1.GET("/health/datasources", handler.CheckDataSourceHealth)
		v1.GET("/health/ai-providers", handler.CheckAIProviderHealth)
		v1.GET("/health/storage", handler.HealthStorage)
		v1.GET("/audit/events", handler.ListAuditEvents)
		v1.GET("/oncall/config", handler.GetOnCallConfig)
		v1.POST("/oncall/config", adminRequired, handler.SaveOnCallConfig)
		v1.GET("/oncall/groups", handler.ListOnCallGroups)
		v1.POST("/oncall/groups", adminRequired, handler.SaveOnCallGroup)
		v1.DELETE("/oncall/groups/:id", adminRequired, handler.DeleteOnCallGroup)
		v1.GET("/oncall/channels", handler.ListOnCallChannels)
		v1.POST("/oncall/channels", adminRequired, handler.SaveOnCallChannel)
		v1.DELETE("/oncall/channels/:id", adminRequired, handler.DeleteOnCallChannel)
		v1.GET("/oncall/schedules", handler.ListOnCallSchedules)
		v1.POST("/oncall/schedules", adminRequired, handler.SaveOnCallSchedule)
		v1.DELETE("/oncall/schedules/:id", adminRequired, handler.DeleteOnCallSchedule)
		v1.GET("/oncall/records", handler.ListOnCallRecords)
		v1.POST("/oncall/test-send", adminRequired, handler.TestOnCallNotification)

		v1.GET("/topology", handler.GetTopology)
		v1.POST("/topology", adminRequired, handler.SaveTopology)
		v1.GET("/topology/resources", handler.TopologyResources)
		v1.POST("/topology/discover", adminRequired, handler.DiscoverTopology)
		v1.POST("/topology/ai/generate", adminRequired, handler.GenerateAITopology)
		v1.GET("/topology/businesses", handler.ListTopologyBusinesses)
		v1.POST("/topology/businesses", adminRequired, handler.SaveTopologyBusiness)
		v1.GET("/topology/businesses/:id", handler.GetTopologyBusiness)
		v1.GET("/topology/businesses/:id/inspect", handler.InspectTopologyBusiness)
		v1.DELETE("/topology/businesses/:id", adminRequired, handler.DeleteTopologyBusiness)

		v1.GET("/ai-providers", handler.GetAIProviders)
		v1.POST("/ai-providers", adminRequired, handler.SaveAIProviders)
		v1.GET("/data-sources", handler.GetDataSources)
		v1.POST("/data-sources", adminRequired, handler.SaveDataSources)

		v1.GET("/credentials", handler.ListCredentials)
		v1.POST("/credentials", adminRequired, handler.SaveCredential)
		v1.DELETE("/credentials/:id", adminRequired, handler.DeleteCredential)

		v1.POST("/auth/login", handler.Login)
		v1.POST("/auth/logout", handler.Logout)
		v1.GET("/auth/me", handler.RequireAuth(), handler.GetMe)
		v1.POST("/auth/change-password", handler.RequireAuth(), handler.ChangePassword)

		v1.GET("/user-profiles", handler.ListUserProfiles)
		v1.POST("/user-profiles", handler.SaveUserProfile)
		v1.DELETE("/user-profiles/:id", handler.DeleteUserProfile)

		v1.GET("/dashboard/summary", handler.DashboardSummary)

		v1.GET("/correlate", handler.CorrelateHandler)
		v1.POST("/workflows/route-preview", handler.RoutePreviewHandler)

		v1.GET("/n9e/agents", handler.ListN9eAgents)
		v1.GET("/n9e/alerts", handler.ListN9eAlerts)

		// 知识库
		v1.GET("/knowledge/cases", adminRequired, handler.ListCases)
		v1.GET("/knowledge/cases/export", adminRequired, handler.ExportCases)
		v1.POST("/knowledge/cases", adminRequired, handler.CreateCase)
		v1.POST("/knowledge/cases/import", adminRequired, handler.ImportCases)
		v1.GET("/knowledge/cases/:id", handler.GetCase)
		v1.PUT("/knowledge/cases/:id", adminRequired, handler.UpdateCase)
		v1.DELETE("/knowledge/cases/:id", adminRequired, handler.DeleteCase)

		// 知识库文档管理
		v1.POST("/knowledge/documents/upload", adminRequired, handler.UploadDocument)
		v1.GET("/knowledge/documents", handler.ListDocumentsHandler)
		v1.GET("/knowledge/documents/:id", handler.GetDocumentHandler)
		v1.DELETE("/knowledge/documents/:id", adminRequired, handler.DeleteDocumentHandler)
		v1.POST("/knowledge/documents/:id/reindex", adminRequired, handler.ReindexDocumentHandler)
		v1.POST("/knowledge/search", handler.SearchKnowledge)
		v1.GET("/knowledge/search/stats", handler.KnowledgeSearchStats)
		v1.POST("/knowledge/search/badcase", adminRequired, handler.SubmitSearchBadcase)
		v1.POST("/knowledge/reindex-all", adminRequired, handler.ReindexAllHandler)

		// 诊断工作流
		v1.POST("/diagnosis/start", adminRequired, handler.StartDiagnosisWorkflow)

		// 工作流管理
		v1.GET("/workflows", handler.ListWorkflows)
		v1.GET("/workflows/:id", handler.GetWorkflow)
		v1.POST("/workflows", adminRequired, handler.CreateWorkflow)
		v1.PUT("/workflows/:id", adminRequired, handler.UpdateWorkflow)
		v1.DELETE("/workflows/:id", adminRequired, handler.DeleteWorkflow)
		v1.POST("/workflows/:id/run", handler.RunWorkflowAPI)
		v1.POST("/workflows/:id/stream", handler.StreamWorkflowAPI)
		v1.GET("/workflows/:id/runs", handler.ListWorkflowRuns)

		// 定时调度
		v1.GET("/schedules", handler.ListSchedulesHandler)
		v1.POST("/schedules", adminRequired, handler.CreateScheduleHandler)
		v1.DELETE("/schedules/:id", adminRequired, handler.DeleteScheduleHandler)

		// 通知渠道
		v1.GET("/notifications/channels", handler.ListNotificationChannels)
		v1.POST("/notifications/channels", adminRequired, handler.SaveNotificationChannel)
		v1.DELETE("/notifications/channels/:id", adminRequired, handler.DeleteNotificationChannel)

		// 指标映射
		v1.POST("/metrics/scan", adminRequired, handler.ScanMetrics)
		v1.POST("/metrics/auto-adapt", adminRequired, handler.AutoAdaptMetrics)
		v1.GET("/metrics/mappings", handler.ListMetricsMappings)
		v1.PUT("/metrics/mappings/:id", adminRequired, handler.UpdateMetricMapping)
		v1.POST("/metrics/mappings/confirm", adminRequired, handler.ConfirmMappings)

		// 知识库 - Runbook
		v1.GET("/knowledge/runbooks", handler.ListRunbooks)
		v1.GET("/knowledge/runbooks/:id", handler.GetRunbook)
		v1.POST("/knowledge/runbooks", adminRequired, handler.CreateRunbook)
		v1.PUT("/knowledge/runbooks/:id", adminRequired, handler.UpdateRunbook)
		v1.DELETE("/knowledge/runbooks/:id", adminRequired, handler.DeleteRunbook)
		v1.POST("/knowledge/runbooks/:id/execute", adminRequired, handler.ExecuteRunbook)
		v1.GET("/knowledge/runbooks/:id/history", handler.ListRunbookHistory)

		// 诊断反馈与归档
		v1.POST("/diagnosis/feedback", handler.SubmitFeedback)
		v1.GET("/diagnosis/feedback", handler.ListFeedbacksByDiagnosisHandler)
		v1.GET("/diagnosis/feedback/all", handler.ListAllFeedbacks)
		v1.GET("/diagnosis/feedback/stats", handler.FeedbackStats)
		v1.GET("/diagnosis/verifications", handler.ListVerifications)
		v1.POST("/diagnosis/archive", adminRequired, handler.ArchiveDiagnosis)

		// Prompt 模板管理
		v1.GET("/prompts", handler.ListPromptTemplates)
		v1.GET("/prompts/:name", handler.GetPromptTemplate)
		v1.POST("/prompts", adminRequired, handler.CreatePromptTemplate)
		v1.PUT("/prompts/:name", adminRequired, handler.UpdatePromptTemplate)
		v1.DELETE("/prompts/:name", adminRequired, handler.DeletePromptTemplate)

		// AI 设置
		v1.GET("/settings/ai", handler.GetAISettings)
		v1.PUT("/settings/ai", adminRequired, handler.UpdateAISettings)
		v1.GET("/settings/ai/:key", handler.GetAISetting)
		v1.PUT("/settings/ai/:key", adminRequired, handler.UpdateAISettingHandler)
		v1.DELETE("/settings/ai/:key", adminRequired, handler.DeleteAISetting)

		// Embedding + Reranker 配置
		v1.GET("/settings/embedding", handler.GetEmbeddingSettings)
		v1.PUT("/settings/embedding", adminRequired, handler.UpdateEmbeddingSettings)
		v1.POST("/settings/embedding/test", adminRequired, handler.TestEmbeddingConnection)
		v1.GET("/settings/reranker", handler.GetRerankerSettings)
		v1.PUT("/settings/reranker", adminRequired, handler.UpdateRerankerSettings)

		// catpaw 二进制内网分发
		r.Static("/download", "./assets")
	}

	host := viper.GetString("server.host")
	if host == "" {
		host = "0.0.0.0"
	}
	port := viper.GetString("server.port")
	addr := host + ":" + port
	logrus.Infof("服务启动，监听: %s", addr)
	r.Run(addr)
}

func corsConfig() cors.Config {
	origins := viper.GetStringSlice("server.allowed_origins")
	cfg := cors.Config{
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization", "X-Admin-Token", "X-Test-Batch-Id"},
		ExposeHeaders:    []string{"X-Security-Mode"},
		AllowCredentials: false,
	}
	if len(origins) == 0 || (len(origins) == 1 && origins[0] == "*") {
		cfg.AllowAllOrigins = true
	} else {
		cfg.AllowOrigins = origins
	}
	return cfg
}

func requireAdminToken() gin.HandlerFunc {
	return requireAdminTokenWithAuth(handler.RequireAuth())
}

func requireAdminTokenWithAuth(authRequired gin.HandlerFunc) gin.HandlerFunc {
	configured := strings.TrimSpace(os.Getenv("AIW_ADMIN_TOKEN"))
	if configured == "" {
		configured = strings.TrimSpace(viper.GetString("security.admin_token"))
	}
	logrus.Infof("admin token configured: len=%d empty=%v", len(configured), configured == "")
	return func(c *gin.Context) {
		if configured == "" {
			c.Header("X-Security-Mode", "permissive-admin-token-not-configured")
			c.Next()
			return
		}
		provided := strings.TrimSpace(c.GetHeader("X-Admin-Token"))
		if provided != "" && provided == configured {
			c.Header("X-Security-Mode", "admin-token-enforced")
			c.Next()
			return
		}
		authorization := strings.TrimSpace(c.GetHeader("Authorization"))
		if !strings.HasPrefix(authorization, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin token required"})
			return
		}
		bearer := strings.TrimSpace(strings.TrimPrefix(authorization, "Bearer "))
		if bearer == configured {
			c.Header("X-Security-Mode", "admin-token-enforced")
			c.Next()
			return
		}
		if bearer != "" {
			c.Header("X-Security-Mode", "login-token-enforced")
			authRequired(c)
			return
		}
		c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "admin token required"})
	}
}
