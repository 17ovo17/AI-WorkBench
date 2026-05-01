package scheduler

import (
	"ai-workbench-api/internal/model"
	"ai-workbench-api/internal/store"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type DiagnosisFunc func(record *model.DiagnoseRecord, prompt string)

var diagnoseFn DiagnosisFunc

func SetDiagnoseFunc(fn DiagnosisFunc) {
	diagnoseFn = fn
}

func Start() {
	intervalHours := viper.GetInt("scheduler.inspection_interval_hours")
	if intervalHours <= 0 {
		intervalHours = 6
	}
	enabled := viper.GetBool("scheduler.enabled")
	if !enabled {
		logrus.Info("scheduler: disabled")
		return
	}
	go runLoop(time.Duration(intervalHours) * time.Hour)
	logrus.Infof("scheduler: started, interval=%dh", intervalHours)
}

func runLoop(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		runScheduledInspection()
	}
}

func runScheduledInspection() {
	if diagnoseFn == nil {
		return
	}
	businesses := store.ListTopologyBusinesses()
	if len(businesses) == 0 {
		logrus.Info("scheduler: no businesses to inspect")
		return
	}
	for _, biz := range businesses {
		if len(biz.Hosts) == 0 {
			continue
		}
		firstHost := biz.Hosts[0]
		if firstHost == "" {
			continue
		}
		record := &model.DiagnoseRecord{
			ID:       store.NewID(),
			TargetIP: firstHost,
			Trigger:  "scheduled",
			Source:   "business_inspection",
			Status:   model.StatusPending,
			CreateTime: time.Now(),
		}
		store.AddRecord(record)
		logrus.Infof("scheduler: inspecting business %s (host=%s)", biz.Name, firstHost)
		go diagnoseFn(record, "定时巡检：请对该主机进行全面健康检查")
	}
}
