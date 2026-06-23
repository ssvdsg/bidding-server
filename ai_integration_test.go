package main

import "testing"

func TestDefaultAIWorkflowWithSyncedBid(t *testing.T) {
	record, err := fetchBidRecordByID("yizhaobiao-36951")
	if err != nil {
		t.Fatalf("fetch bid failed: %v", err)
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
}
