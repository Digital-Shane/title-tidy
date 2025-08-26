package log

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestLogSession(t *testing.T) {
	// Test session creation
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	loggingEnabled = true
	
	err := StartSession("test", []string{"arg1", "arg2"})
	if err != nil {
		t.Fatalf("StartSession() failed: %v", err)
	}
	
	if currentSession == nil {
		t.Fatal("StartSession() should have created a session")
	}
	
	// Test that session has correct metadata
	meta := currentSession.Metadata
	if meta.CommandArgs[0] != "test" {
		t.Errorf("Expected command 'test', got %s", meta.CommandArgs[0])
	}
	
	if len(meta.CommandArgs) != 3 || meta.CommandArgs[1] != "arg1" || meta.CommandArgs[2] != "arg2" {
		t.Errorf("Expected args ['test', 'arg1', 'arg2'], got %v", meta.CommandArgs)
	}
}

func TestLogOperations(t *testing.T) {
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	loggingEnabled = true
	
	// Start a session
	err := StartSession("test", []string{})
	if err != nil {
		t.Fatalf("StartSession() failed: %v", err)
	}
	
	// Test different operation types
	LogRename("old.txt", "new.txt", true, nil)
	LogLink("source.txt", "link.txt", true, nil)
	LogDelete("delete.txt", true, nil)
	LogCreateDir("newdir", true, nil)
	
	// Test operation with error
	LogRename("error.txt", "failed.txt", false, os.ErrPermission)
	
	if len(currentSession.Operations) != 5 {
		t.Errorf("Expected 5 operations, got %d", len(currentSession.Operations))
	}
	
	// Check operation types
	expectedTypes := []OperationType{OpRename, OpLink, OpDelete, OpCreateDir, OpRename}
	for i, op := range currentSession.Operations {
		if op.Type != expectedTypes[i] {
			t.Errorf("Operation %d: expected type %s, got %s", i, expectedTypes[i], op.Type)
		}
	}

	// Stats are normally saved at the end, but run them now so the unit test does
	// not save a file
	updateStats()
	
	// Check success/failure tracking
	if currentSession.Metadata.SuccessfulOps != 4 {
		t.Errorf("Expected 4 successful operations, got %d", currentSession.Metadata.SuccessfulOps)
	}
	
	if currentSession.Metadata.FailedOps != 1 {
		t.Errorf("Expected 1 failed operation, got %d", currentSession.Metadata.FailedOps)
	}
	
	// Check error handling
	errorOp := currentSession.Operations[4]
	if errorOp.Success {
		t.Error("Expected error operation to be marked as failed")
	}
	
	if errorOp.Error == "" {
		t.Error("Expected error operation to have error message")
	}
}

func TestSessionSerialization(t *testing.T) {
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	// Create temporary directory for test
	tempDir := t.TempDir()
	
	// Create a mock session
	session := &LogSession{
		Metadata: SessionMetadata{
			CommandArgs:    []string{"test", "arg1"},
			WorkingDir:     tempDir,
			Timestamp:      time.Now(),
			SessionID:      "test_session_123",
			TotalOps:       2,
			SuccessfulOps:  1,
			FailedOps:      1,
		},
		Operations: []OperationLog{
			{
				ID:         "test_session_123_0",
				Timestamp:  time.Now(),
				Type:       OpRename,
				SourcePath: "old.txt",
				DestPath:   "new.txt",
				Success:    true,
			},
			{
				ID:         "test_session_123_1",
				Timestamp:  time.Now(),
				Type:       OpDelete,
				SourcePath: "delete.txt",
				Success:    false,
				Error:      "file not found",
			},
		},
	}
	
	// Test write and read
	testFile := filepath.Join(tempDir, "test_session.json")
	err := WriteSessionToPath(session, testFile)
	if err != nil {
		t.Fatalf("WriteSessionToPath() failed: %v", err)
	}
	
	readSession, err := ReadSession(testFile)
	if err != nil {
		t.Fatalf("ReadSession() failed: %v", err)
	}
	
	// Compare sessions
	if diff := cmp.Diff(session, readSession); diff != "" {
		t.Errorf("Session mismatch (-want +got):\n%s", diff)
	}
}

func TestLoggingDisabled(t *testing.T) {
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	// Disable logging
	loggingEnabled = false
	
	err := StartSession("test", []string{})
	if err != nil {
		t.Fatalf("StartSession() failed: %v", err)
	}
	
	if currentSession != nil {
		t.Error("Session should not be created when logging is disabled")
	}
	
	// Operations should be no-ops
	LogRename("old.txt", "new.txt", true, nil)
	
	if currentSession != nil {
		t.Error("Operations should not create session when logging disabled")
	}
}

// Helper function to write session to specific path for testing
func WriteSessionToPath(session *LogSession, path string) error {
	// Create directory if needed
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	
	// Use the existing WriteSession logic but write to specific path
	data, err := session.toJSON()
	if err != nil {
		return err
	}
	
	return os.WriteFile(path, data, 0644)
}

// Helper method for JSON marshaling
func (s *LogSession) toJSON() ([]byte, error) {
	return json.MarshalIndent(s, "", "  ")
}

func TestInitialize(t *testing.T) {
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	// Test initialization with logging enabled
	Initialize(true, 30)
	
	if !loggingEnabled {
		t.Error("Logging should be enabled after Initialize(true, 30)")
	}
	
	// Test initialization with logging disabled
	Initialize(false, 30)
	
	if loggingEnabled {
		t.Error("Logging should be disabled after Initialize(false, 30)")
	}
	
	// Verify that session creation respects the setting
	err := StartSession("test", []string{})
	if err != nil {
		t.Fatalf("StartSession() failed: %v", err)
	}
	
	if currentSession != nil {
		t.Error("Session should not be created when logging is disabled")
	}
}

func TestStartSessionWhenDisabled(t *testing.T) {
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	Initialize(false, 30) // logging disabled
	
	err := StartSession("test", []string{})
	if err != nil {
		t.Errorf("StartSession() with logging disabled error = %v, want nil", err)
	}
	
	if currentSession != nil {
		t.Error("StartSession() with logging disabled should not set currentSession")
	}
}

func TestEndSessionWhenDisabled(t *testing.T) {
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	Initialize(false, 30) // logging disabled
	
	err := EndSession()
	if err != nil {
		t.Errorf("EndSession() with logging disabled error = %v, want nil", err)
	}
}

func TestEndSessionWithNilSession(t *testing.T) {
	originalLoggingEnabled := loggingEnabled
	defer func() { 
		loggingEnabled = originalLoggingEnabled
		currentSession = nil 
	}()
	
	Initialize(true, 30) // logging enabled
	currentSession = nil // but no active session
	
	err := EndSession()
	if err != nil {
		t.Errorf("EndSession() with nil session error = %v, want nil", err)
	}
}