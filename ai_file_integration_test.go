package main

import "testing"

func TestDefaultAIWorkflowWithPDFAttachment(t *testing.T) {
	record, err := fetchBidRecordByID("e8edfc13-9bbc-481e-b18c-f3cd4df769d0")
	if err != nil {
		t.Fatalf("fetch bid failed: %v", err)
	}
	if !record.PDFURL.Valid || record.PDFURL.String == "" {
		t.Fatal("pdf url is empty")
	}

	result, err := performBidAnalysis(record, "", "auto", "")
	if err != nil {
		t.Fatalf("perform bid analysis failed: %v", err)
	}
	if result == nil {
		t.Fatal("analysis result is nil")
	}
	if result.AIModel == "" {
		t.Fatal("ai model is empty")
	}
	if len(result.Analysis) == 0 {
		t.Fatal("analysis is empty")
	}
	if result.AIModel != "Relay AI File (TEXT_DEEPSEEK_V4)" {
		t.Fatalf("unexpected ai model: %s", result.AIModel)
	}
}
