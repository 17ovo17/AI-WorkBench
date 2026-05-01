package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	"ai-workbench-api/internal/model"

	"github.com/spf13/viper"
)

func pushToPushgateway(alert *model.AlertRecord) {
	var ds []model.DataSource
	viper.UnmarshalKey("data_sources", &ds)
	var pgURL string
	for _, d := range ds {
		if d.Type == "pushgateway" && d.URL != "" {
			pgURL = strings.TrimRight(d.URL, "/")
			break
		}
	}
	if pgURL == "" {
		return
	}

	severity := alert.Severity
	if severity == "" {
		severity = "warning"
	}
	status := 1
	if alert.Status == "resolved" {
		status = 0
	}

	// Prometheus text format
	job := "catpaw_alert"
	instance := alert.TargetIP
	body := fmt.Sprintf(`# HELP catpaw_alert_firing Alert firing status from catpaw
# TYPE catpaw_alert_firing gauge
catpaw_alert_firing{alertname=%q,severity=%q,instance=%q} %d
`, alert.Title, severity, instance, status)

	url := fmt.Sprintf("%s/metrics/job/%s/instance/%s", pgURL, job, instance)
	req, _ := http.NewRequest("POST", url, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "text/plain")
	http.DefaultClient.Do(req)
}
