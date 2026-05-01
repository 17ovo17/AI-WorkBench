package handler

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"

	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"github.com/spf13/viper"
)

func Metrics(c *gin.Context) {
	storage := store.Health()
	mysqlUp := 0
	redisUp := 0
	if ok, _ := storage["mysql"].(bool); ok {
		mysqlUp = 1
	}
	if ok, _ := storage["redis"].(bool); ok {
		redisUp = 1
	}
	body := fmt.Sprintf(`# HELP ai_workbench_up AI WorkBench API process status.
# TYPE ai_workbench_up gauge
ai_workbench_up 1
# HELP ai_workbench_storage_up AI WorkBench storage dependency status.
# TYPE ai_workbench_storage_up gauge
ai_workbench_storage_up{component="mysql"} %d
ai_workbench_storage_up{component="redis"} %d
`, mysqlUp, redisUp)
	c.Data(http.StatusOK, "text/plain; version=0.0.4; charset=utf-8", []byte(body))
}

func CheckDataSourceHealth(c *gin.Context) {
	ds := loadDataSources()
	results := make([]map[string]interface{}, len(ds))
	client := &http.Client{Timeout: 3 * time.Second}
	for i, d := range ds {
		alive := false
		detail := ""
		switch d.Type {
		case "prometheus", "pushgateway":
			base := strings.TrimRight(strings.TrimSpace(d.URL), "/")
			resp, err := client.Get(base + "/-/ready")
			alive = err == nil && resp != nil && resp.StatusCode == http.StatusOK
			if err != nil {
				detail = err.Error()
			} else if resp != nil {
				detail = resp.Status
				_ = resp.Body.Close()
			}
			if !alive && d.Type == "prometheus" && base != "" {
				resp, err = client.Get(base + "/api/v1/query?query=up")
				alive = err == nil && resp != nil && resp.StatusCode == http.StatusOK
				if err != nil {
					detail = err.Error()
				} else if resp != nil {
					detail = "query up: " + resp.Status
					_ = resp.Body.Close()
				}
			}
		case "mysql":
			dsn := mysqlDSN(d)
			alive, detail = pingSQL("mysql", dsn)
		default:
			if strings.TrimSpace(d.URL) != "" {
				resp, err := client.Get(d.URL)
				alive = err == nil && resp != nil && resp.StatusCode < http.StatusInternalServerError
				if err != nil {
					detail = err.Error()
				} else if resp != nil {
					detail = resp.Status
				}
			} else {
				detail = "data source url is empty"
			}
		}
		status := "error"
		if alive {
			status = "healthy"
		}
		results[i] = map[string]interface{}{"id": d.ID, "name": d.Name, "type": d.Type, "url": d.URL, "alive": alive, "status": status, "detail": detail}
	}
	c.JSON(http.StatusOK, results)
}

func CheckAIProviderHealth(c *gin.Context) {
	providers := loadAIProviders()
	results := make([]map[string]interface{}, len(providers))
	client := &http.Client{Timeout: 5 * time.Second}
	for i, p := range providers {
		alive := false
		detail := "not checked"
		baseURL := strings.TrimSuffix(strings.TrimRight(strings.TrimSpace(p.BaseURL), "/"), "/chat/completions")
		apiKey := strings.TrimSpace(p.APIKey)
		if baseURL == "" {
			detail = "base_url is empty"
		} else if apiKey == "" || apiKey == "******" || strings.Contains(apiKey, "${") {
			detail = "api_key is empty or unresolved placeholder"
		} else {
			req, _ := http.NewRequest("GET", baseURL+"/models", nil)
			req.Header.Set("Authorization", "Bearer "+apiKey)
			resp, err := client.Do(req)
			if err != nil {
				detail = err.Error()
			} else if resp != nil {
				alive = resp.StatusCode >= 200 && resp.StatusCode < 400
				detail = resp.Status
				_ = resp.Body.Close()
			}
		}
		results[i] = map[string]interface{}{"id": p.ID, "alive": alive, "detail": detail}
	}
	c.JSON(http.StatusOK, results)
}

func mysqlDSN(d model.DataSource) string {
	if d.ID == "platform-mysql" {
		return strings.TrimSpace(viper.GetString("mysql.dsn"))
	}
	if strings.Contains(d.URL, "@tcp(") || strings.Contains(d.URL, "@unix(") {
		return d.URL
	}
	if strings.TrimSpace(d.URL) == "" && strings.TrimSpace(d.Username) == "" {
		return strings.TrimSpace(viper.GetString("mysql.dsn"))
	}
	host := strings.TrimSpace(d.URL)
	if host == "" {
		host = mysqlHostFromDSN(viper.GetString("mysql.dsn"))
	}
	if host == "" {
		host = "127.0.0.1:3306"
	}
	database := strings.TrimSpace(d.Database)
	if database == "" {
		database = "ai_workbench"
	}
	return fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&parseTime=true&loc=Local", d.Username, d.Password, host, database)
}

func mysqlHostFromDSN(dsn string) string {
	start := strings.Index(dsn, "@tcp(")
	if start < 0 {
		return ""
	}
	start += len("@tcp(")
	end := strings.Index(dsn[start:], ")")
	if end < 0 {
		return ""
	}
	return dsn[start : start+end]
}

func pingSQL(driver, dsn string) (bool, string) {
	if strings.TrimSpace(dsn) == "" {
		return false, "dsn is empty"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return false, err.Error()
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		return false, err.Error()
	}
	return true, "ping ok"
}
