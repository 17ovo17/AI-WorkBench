package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
)

func GetPrometheusInstances(c *gin.Context) {
	promURL := viper.GetString("prometheus.url")
	if promURL == "" {
		// 从数据源配置里找
		var ds []struct {
			Type string `mapstructure:"type"`
			URL  string `mapstructure:"url"`
		}
		viper.UnmarshalKey("data_sources", &ds)
		for _, d := range ds {
			if d.Type == "prometheus" && d.URL != "" {
				promURL = d.URL
				break
			}
		}
	}
	if promURL == "" {
		c.JSON(http.StatusOK, gin.H{"data": []string{}})
		return
	}

	seen := map[string]bool{}
	var all []string
	for _, label := range []string{"instance", "ident"} {
		resp, err := http.Get(promURL + "/api/v1/label/" + label + "/values")
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		var result struct {
			Data []string `json:"data"`
		}
		if json.Unmarshal(body, &result) == nil {
			for _, v := range result.Data {
				if !seen[v] {
					seen[v] = true
					all = append(all, v)
				}
			}
		}
	}
	ensureDefaultMonitoringBusiness(ipsFromPrometheusInstances(all))
	c.JSON(http.StatusOK, gin.H{"status": "success", "data": all})
}

func ipsFromPrometheusInstances(values []string) []string {
	seen := map[string]bool{}
	ips := []string{}
	for _, value := range values {
		for _, ip := range ipRe.FindAllString(value, -1) {
			if !seen[ip] {
				seen[ip] = true
				ips = append(ips, ip)
			}
		}
	}
	return ips
}
