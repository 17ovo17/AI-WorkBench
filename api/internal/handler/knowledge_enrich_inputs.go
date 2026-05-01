package handler

func buildKnowledgeEnrichInputs(diagnosisID string) map[string]any {
	inputs := map[string]any{"diagnosis_id": diagnosisID}
	rec := findDiagnoseByID(diagnosisID)
	if rec == nil {
		return inputs
	}

	report := rec.Report
	if rec.SummaryReport != "" && rec.Report != "" {
		report = rec.SummaryReport + "\n\n" + rec.Report
	} else if report == "" {
		report = rec.SummaryReport
	}
	if report == "" {
		report = rec.RawReport
	}

	inputs["diagnosis_report"] = report
	inputs["target_ip"] = rec.TargetIP
	inputs["alert_title"] = rec.AlertTitle
	inputs["status"] = string(rec.Status)
	inputs["source"] = rec.Source
	inputs["data_source"] = rec.DataSource
	return inputs
}
