package task

import (
	"context"
	"testing"
)

// mockLLMClient is defined in planner_test.go

func TestParseCompletionVerdict_Completed(t *testing.T) {
	verdict, reason := parseCompletionVerdict("COMPLETED\nAll files were created successfully.")
	if verdict != VerdictCompleted {
		t.Errorf("expected COMPLETED, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestParseCompletionVerdict_Stuck(t *testing.T) {
	verdict, reason := parseCompletionVerdict("STUCK\nAgent could not resolve import errors.")
	if verdict != VerdictStuck {
		t.Errorf("expected STUCK, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestParseCompletionVerdict_Failed(t *testing.T) {
	verdict, _ := parseCompletionVerdict("FAILED\nFatal build error.")
	if verdict != VerdictFailed {
		t.Errorf("expected FAILED, got %q", verdict)
	}
}

func TestParseCompletionVerdict_Lowercase(t *testing.T) {
	verdict, _ := parseCompletionVerdict("completed\nlooks good")
	if verdict != VerdictCompleted {
		t.Errorf("expected COMPLETED, got %q", verdict)
	}
}

func TestParseCompletionVerdict_SingleLine(t *testing.T) {
	verdict, reason := parseCompletionVerdict("STUCK")
	if verdict != VerdictStuck {
		t.Errorf("expected STUCK, got %q", verdict)
	}
	if reason != "" {
		t.Errorf("expected empty reason, got %q", reason)
	}
}

func TestParseCompletionVerdict_Unknown(t *testing.T) {
	verdict, reason := parseCompletionVerdict("MAYBE\nuncertain")
	if verdict != VerdictCompleted {
		t.Errorf("expected default COMPLETED, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestParseVerificationVerdict_Passed(t *testing.T) {
	verdict, reason := parseVerificationVerdict("PASSED\nAll checks passed.")
	if verdict != VerifyPassed {
		t.Errorf("expected PASSED, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestParseVerificationVerdict_Failed(t *testing.T) {
	verdict, reason := parseVerificationVerdict("FAILED\nMissing error handling in auth.go")
	if verdict != VerifyFailed {
		t.Errorf("expected FAILED, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestParseVerificationVerdict_DefaultFailed(t *testing.T) {
	verdict, _ := parseVerificationVerdict("SOMETHING ELSE")
	if verdict != VerifyFailed {
		t.Errorf("expected default FAILED, got %q", verdict)
	}
}

func TestJudgeCodingComplete_NoClient(t *testing.T) {
	verdict, reason := JudgeCodingComplete(context.Background(), nil, "Test", "desc", "output")
	if verdict != VerdictCompleted {
		t.Errorf("expected COMPLETED when no client, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestJudgeCodingComplete_EmptyOutput(t *testing.T) {
	verdict, reason := JudgeCodingComplete(context.Background(), &mockLLMClient{}, "Test", "desc", "")
	if verdict != VerdictStuck {
		t.Errorf("expected STUCK for empty output, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestJudgeCodingComplete_WithMock(t *testing.T) {
	client := &mockLLMClient{response: "COMPLETED\nAll good"}
	verdict, reason := JudgeCodingComplete(context.Background(), client, "Test", "desc", "some agent output")
	if verdict != VerdictCompleted {
		t.Errorf("expected COMPLETED, got %q", verdict)
	}
	if reason != "All good" {
		t.Errorf("expected reason 'All good', got %q", reason)
	}
}

func TestJudgeVerification_NoClient(t *testing.T) {
	verdict, reason := JudgeVerification(context.Background(), nil, "Test", "output")
	if verdict != VerifyPassed {
		t.Errorf("expected PASSED when no client, got %q", verdict)
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestJudgeVerification_EmptyOutput(t *testing.T) {
	verdict, _ := JudgeVerification(context.Background(), &mockLLMClient{}, "Test", "")
	if verdict != VerifyFailed {
		t.Errorf("expected FAILED for empty output, got %q", verdict)
	}
}

func TestJudgeVerification_WithMock(t *testing.T) {
	client := &mockLLMClient{response: "PASSED\nNo issues found"}
	verdict, reason := JudgeVerification(context.Background(), client, "Test", "some verification output")
	if verdict != VerifyPassed {
		t.Errorf("expected PASSED, got %q", verdict)
	}
	if reason != "No issues found" {
		t.Errorf("expected reason 'No issues found', got %q", reason)
	}
}

func TestTruncateForJudge_Short(t *testing.T) {
	result := truncateForJudge("hello", 100)
	if result != "hello" {
		t.Errorf("expected 'hello', got %q", result)
	}
}

func TestTruncateForJudge_Long(t *testing.T) {
	long := ""
	for i := 0; i < 100; i++ {
		long += "x"
	}
	result := truncateForJudge(long, 50)
	if len(result) <= 50 {
		t.Error("truncated result should include prefix")
	}
}

func TestVerdictConstants(t *testing.T) {
	if VerdictCompleted != "COMPLETED" {
		t.Error("VerdictCompleted mismatch")
	}
	if VerdictStuck != "STUCK" {
		t.Error("VerdictStuck mismatch")
	}
	if VerdictFailed != "FAILED" {
		t.Error("VerdictFailed mismatch")
	}
	if VerifyPassed != "PASSED" {
		t.Error("VerifyPassed mismatch")
	}
	if VerifyFailed != "FAILED" {
		t.Error("VerifyFailed mismatch")
	}
}
