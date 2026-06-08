package task

import (
	"testing"
)

func TestParsePlans_RealOpencodeOutput(t *testing.T) {
	// This is exactly what opencode outputs for plan mode
	response := "```json\n[\n  {\"title\": \"Backend - User Model\", \"description\": \"Create user table\", \"directory\": \"/tmp/backend\"},\n  {\"title\": \"Frontend - Login Form\", \"description\": \"Build login form UI\", \"directory\": \"/tmp/frontend\"}\n]\n```"

	plans, err := ParsePlans(response)
	if err != nil {
		t.Fatalf("ParsePlans failed: %v", err)
	}
	if len(plans) != 2 {
		t.Fatalf("expected 2 plans, got %d", len(plans))
	}
	if plans[0].Title != "Backend - User Model" {
		t.Errorf("unexpected title: %q", plans[0].Title)
	}
	if plans[0].Directory != "/tmp/backend" {
		t.Errorf("unexpected directory: %q", plans[0].Directory)
	}
	if plans[1].Title != "Frontend - Login Form" {
		t.Errorf("unexpected title: %q", plans[1].Title)
	}
}
