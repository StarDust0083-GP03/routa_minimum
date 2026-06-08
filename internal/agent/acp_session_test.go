package agent

import (
	"context"
	"testing"
	"time"

	"codeg/internal/acp"
)

func TestNewAcpSession(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	params := SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project/test",
		Prompt:    "Write code",
	}

	session, err := NewAcpSession(client, params)
	if err != nil {
		t.Fatalf("NewAcpSession failed: %v", err)
	}
	if session == nil {
		t.Fatal("expected non-nil session")
	}
	if session.State() != acp.SessionInitializing {
		t.Errorf("expected state 'initializing', got %q", session.State())
	}
	if session.Phase() != "coding" {
		t.Errorf("expected phase 'coding', got %q", session.Phase())
	}
	if session.SubTaskID() != "st-1" {
		t.Errorf("expected subTaskID 'st-1', got %q", session.SubTaskID())
	}
}

func TestNewAcpSession_NilClient(t *testing.T) {
	_, err := NewAcpSession(nil, SessionParams{Cwd: "/test"})
	if err == nil {
		t.Error("expected error for nil client")
	}
}

func TestNewAcpSession_NoCwd(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	_, err := NewAcpSession(client, SessionParams{})
	if err == nil {
		t.Error("expected error for empty cwd")
	}
}

func TestAcpSession_ID(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
	})

	// ID should be empty before Start
	if session.ID() != "" {
		t.Errorf("expected empty ID before start, got %q", session.ID())
	}
}

func TestAcpSession_IsActive(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
	})

	if session.IsActive() {
		t.Error("session should not be active before Start")
	}
}

func TestAcpSession_Events(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
	})

	ch := session.Events()
	if ch == nil {
		t.Error("expected non-nil events channel")
	}
}

func TestAcpSession_StartedAt(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
	})

	if !session.StartedAt().IsZero() {
		t.Error("expected zero StartedAt before Start")
	}
}

func TestAcpSession_CompletedAt(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
	})

	if session.CompletedAt() != nil {
		t.Error("expected nil CompletedAt before Start")
	}
}

func TestAcpSession_Start_InvalidState(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
	})

	// Start the session (will fail because no server, but state changes to error)
	_ = session.Start(context.Background())

	// Trying to Start again should fail
	err := session.Start(context.Background())
	if err == nil {
		t.Error("expected error when starting session in non-initializing state")
	}
}

func TestAcpSession_Cancel_BeforeStart(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
	})

	// Cancel before Start should not panic
	err := session.Cancel()
	if err != nil {
		t.Logf("Cancel before start: %v (expected if no server)", err)
	}
}

func TestAcpSession_Start_NoServer(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-1",
		Phase:     "coding",
		Role:      acp.RoleDeveloper,
		Cwd:       "/project",
		Prompt:    "test prompt",
	})

	// This should fail because no server is running
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := session.Start(ctx)
	if err == nil {
		t.Error("expected error when starting with no server")
	}
}

func TestSessionParams(t *testing.T) {
	params := SessionParams{
		SubTaskID: "st-1",
		Phase:     "verifying",
		Role:      acp.RoleGate,
		Cwd:       "/project/verify",
		Prompt:    "Verify the implementation",
		Name:      "verification-session",
	}

	if params.SubTaskID != "st-1" {
		t.Error("SubTaskID mismatch")
	}
	if params.Phase != "verifying" {
		t.Error("Phase mismatch")
	}
	if params.Role != acp.RoleGate {
		t.Error("Role mismatch")
	}
}

func TestAcpSession_PhaseAndSubTaskID(t *testing.T) {
	client := acp.NewACPClient("http://localhost:9999/api/acp")
	session, _ := NewAcpSession(client, SessionParams{
		SubTaskID: "st-verify",
		Phase:     "verifying",
		Role:      acp.RoleGate,
		Cwd:       "/project",
	})

	if session.Phase() != "verifying" {
		t.Errorf("expected phase 'verifying', got %q", session.Phase())
	}
	if session.SubTaskID() != "st-verify" {
		t.Errorf("expected subTaskID 'st-verify', got %q", session.SubTaskID())
	}
}
