package handler

import (
	"strings"
	"testing"
	"time"

	"ai-workbench-api/internal/model"
)

func TestRichInspectionReportContainsOpsStructure(t *testing.T) {
	inspection := model.BusinessInspection{
		BusinessName: "AI WorkBench",
		Status:       "critical",
		Score:        42,
		GeneratedAt:  time.Date(2026, 5, 1, 10, 30, 0, 0, time.Local),
		Metrics: []model.BusinessMetricSample{
			{IP: "10.10.1.21", Name: "CPU 使用率", Value: 96, Unit: "%", Status: "critical", Detail: "正常范围 <80%；1小时前 90%；趋势=持续升高"},
			{IP: "10.10.1.21", Name: "内存使用率", Value: 70, Unit: "%", Status: "healthy", Detail: "趋势=稳定"},
			{IP: "10.10.1.21", Name: "磁盘使用率", Value: 40, Unit: "%", Status: "healthy", Detail: "趋势=稳定"},
			{IP: "10.10.1.21", Name: "系统负载(1m)", Value: 0.77, Status: "healthy", Detail: "趋势=稳定"},
			{IP: "10.10.1.21", Name: "TCP 连接数", Value: 9800, Status: "critical", Detail: "趋势=持续升高"},
			{IP: "10.10.1.21", Name: "网络错误率", Value: 0, Unit: "%", Status: "healthy", Detail: "趋势=稳定"},
		},
		Processes: []model.BusinessProcess{{IP: "10.10.1.21", Name: "api-primary", Port: 8080, Layer: "app", Status: "running"}},
	}
	report := renderRichInspectionReport(inspection)
	mustContain := []string{
		"# 业务巡检报告 - AI WorkBench",
		"## 总体评估",
		"## 主机巡检明细",
		"| CPU 使用率 | 96% | <80% | 危险 | 持续升高 |",
		"## 异常汇总",
		"10.10.1.21 | CPU 使用率 | 96% | <80% | critical",
		"## 处置建议",
		"ssh 10.10.1.21 \"top -bn1 | head -20\"",
		"## 历史对比",
	}
	for _, want := range mustContain {
		if !strings.Contains(report, want) {
			t.Fatalf("report missing %q\n%s", want, report)
		}
	}
}

func TestInspectionReasoningIncludesAllSubchecks(t *testing.T) {
	inspection := model.BusinessInspection{Metrics: []model.BusinessMetricSample{
		{IP: "10.10.1.21", Name: "CPU 使用率", Status: "healthy"},
		{IP: "10.10.1.21", Name: "内存使用率", Status: "healthy"},
		{IP: "10.10.1.21", Name: "磁盘使用率", Status: "healthy"},
		{IP: "10.10.1.21", Name: "系统负载(1m)", Status: "healthy"},
		{IP: "10.10.1.21", Name: "TCP 连接数", Status: "healthy"},
		{IP: "10.10.1.21", Name: "网络错误率", Status: "healthy"},
		{IP: "10.10.1.21", Name: "进程总数", Status: "healthy"},
	}}
	steps := appendInspectionReasoningSteps(nil, inspection)
	wantActions := []string{"inspection_alive", "inspection_cpu", "inspection_memory", "inspection_disk", "inspection_load", "inspection_network", "inspection_process"}
	for _, want := range wantActions {
		found := false
		for _, step := range steps {
			found = found || step.Action == want
		}
		if !found {
			t.Fatalf("missing reasoning step %s in %+v", want, steps)
		}
	}
}
