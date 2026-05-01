package handler

import (
	"strconv"
	"strings"
	"time"

	"ai-workbench-api/internal/metrics"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

const defaultMetricsAutoSyncInterval = 30 * time.Minute

func StartMetricsAutoSync() {
	if !metricsAutoSyncEnabled() {
		logrus.Info("metrics auto sync: disabled")
		return
	}
	go func() {
		runMetricsAutoSyncOnce()
		ticker := time.NewTicker(metricsAutoSyncInterval())
		defer ticker.Stop()
		for range ticker.C {
			runMetricsAutoSyncOnce()
		}
	}()
	logrus.Infof("metrics auto sync: started, interval=%s", metricsAutoSyncInterval())
}

func runMetricsAutoSyncOnce() {
	for _, ds := range prometheusDataSourcesForAutoSync() {
		added, err := metrics.ScanPrometheusMetrics(ds.URL, ds.ID)
		if err != nil {
			logrus.Warnf("metrics auto sync: scan datasource %s failed: %v", ds.ID, err)
			continue
		}
		if added == 0 {
			continue
		}
		prompt, err := loadAdaptPrompt()
		if err != nil {
			logrus.Warnf("metrics auto sync: load prompt failed: %v", err)
			continue
		}
		processed, adapted := runAutoAdapt(ds.ID, prompt, metricsAutoSyncMaxBatches())
		logrus.Infof("metrics auto sync: datasource=%s added=%d processed=%d adapted=%d", ds.ID, added, processed, adapted)
	}
}

func prometheusDataSourcesForAutoSync() []struct{ ID, URL string } {
	out := []struct{ ID, URL string }{}
	var sources []struct {
		ID   string `mapstructure:"id"`
		Type string `mapstructure:"type"`
		URL  string `mapstructure:"url"`
	}
	_ = viper.UnmarshalKey("data_sources", &sources)
	for _, source := range sources {
		if !strings.EqualFold(source.Type, "prometheus") || strings.TrimSpace(source.URL) == "" {
			continue
		}
		out = append(out, struct{ ID, URL string }{ID: firstNonEmpty(source.ID, "prometheus-default"), URL: source.URL})
	}
	if len(out) == 0 && strings.TrimSpace(viper.GetString("prometheus.url")) != "" {
		out = append(out, struct{ ID, URL string }{ID: "prometheus-default", URL: viper.GetString("prometheus.url")})
	}
	return out
}

func metricsAutoSyncEnabled() bool {
	if viper.IsSet("metrics.auto_sync_enabled") {
		return viper.GetBool("metrics.auto_sync_enabled")
	}
	return true
}

func metricsAutoSyncInterval() time.Duration {
	seconds := viper.GetInt("metrics.auto_sync_interval_seconds")
	if seconds <= 0 {
		seconds = int(defaultMetricsAutoSyncInterval / time.Second)
	}
	return time.Duration(seconds) * time.Second
}

func metricsAutoSyncMaxBatches() int {
	value := strings.TrimSpace(viper.GetString("metrics.auto_sync_max_batches"))
	if value == "" {
		return 2
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 || parsed > 20 {
		return 2
	}
	return parsed
}
