package log

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type OperationType string

const (
	OpRename    OperationType = "rename"
	OpLink      OperationType = "link"
	OpDelete    OperationType = "delete"
	OpCreateDir OperationType = "create_dir"
)

type OperationLog struct {
	ID         string        `json:"id"`
	Timestamp  time.Time     `json:"timestamp"`
	Type       OperationType `json:"type"`
	SourcePath string        `json:"source_path"`
	DestPath   string        `json:"dest_path,omitempty"`
	Success    bool          `json:"success"`
	Error      string        `json:"error,omitempty"`
}

type SessionMetadata struct {
	CommandArgs    []string  `json:"command_args"`
	WorkingDir     string    `json:"working_dir"`
	Timestamp      time.Time `json:"timestamp"`
	SessionID      string    `json:"session_id"`
	TotalOps       int       `json:"total_operations"`
	SuccessfulOps  int       `json:"successful_operations"`
	FailedOps      int       `json:"failed_operations"`
}

type LogSession struct {
	Metadata   SessionMetadata `json:"metadata"`
	Operations []OperationLog  `json:"operations"`
}

// Global singleton session manager
var (
	currentSession *LogSession
	sessionMutex   sync.Mutex
	loggingEnabled = true
)

// StartSession initializes a new logging session
func StartSession(command string, args []string) error {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	if !loggingEnabled {
		return nil
	}
	
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get working directory: %w", err)
	}
	
	now := time.Now()
	sessionID := fmt.Sprintf("%s_%s", now.Format("20060102_150405"), fmt.Sprintf("%03d", now.Nanosecond()/1000000))
	
	currentSession = &LogSession{
		Metadata: SessionMetadata{
			CommandArgs: append([]string{command}, args...),
			WorkingDir:  wd,
			Timestamp:   now,
			SessionID:   sessionID,
		},
		Operations: []OperationLog{},
	}
	
	return nil
}

// EndSession saves the current session to disk
func EndSession() error {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	if !loggingEnabled || currentSession == nil {
		return nil
	}

	updateStats()
	err := WriteSession(currentSession)
	currentSession = nil
	return err
}

// LogRename logs a rename operation
func LogRename(sourcePath, destPath string, success bool, err error) {
	LogOperation(OpRename, sourcePath, destPath, success, err)
}

// LogLink logs a link operation
func LogLink(sourcePath, destPath string, success bool, err error) {
	LogOperation(OpLink, sourcePath, destPath, success, err)
}

// LogDelete logs a delete operation
func LogDelete(path string, success bool, err error) {
	LogOperation(OpDelete, path, "", success, err)
}

// LogCreateDir logs a directory creation
func LogCreateDir(dirPath string, success bool, err error) {
	LogOperation(OpCreateDir, "", dirPath, success, err)
}

// LogOperation logs a generic operation to the current session
func LogOperation(opType OperationType, sourcePath, destPath string, success bool, err error) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	if !loggingEnabled || currentSession == nil {
		return
	}
	
	op := OperationLog{
		ID:         fmt.Sprintf("%s_%d", currentSession.Metadata.SessionID, len(currentSession.Operations)),
		Timestamp:  time.Now(),
		Type:       opType,
		SourcePath: sourcePath,
		DestPath:   destPath,
		Success:    success,
	}
	
	if err != nil {
		op.Error = err.Error()
	}
	
	currentSession.Operations = append(currentSession.Operations, op)
}

// updateStats updates the session statistics
func updateStats() {
	if currentSession == nil {
		return
	}
	
	successful := 0
	failed := 0
	
	for _, op := range currentSession.Operations {
		if op.Success {
			successful++
		} else {
			failed++
		}
	}
	
	currentSession.Metadata.TotalOps = len(currentSession.Operations)
	currentSession.Metadata.SuccessfulOps = successful
	currentSession.Metadata.FailedOps = failed
}

// Initialize sets up the logging system with the given configuration
func Initialize(enabled bool, retentionDays int) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	
	loggingEnabled = enabled
	
	if enabled {
		// Clean up old logs on initialization
		if err := cleanupOldLogsUnsafe(retentionDays); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to clean up old logs: %v\n", err)
		}
	}
}

func GetLogPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	logDir := filepath.Join(homeDir, ".title-tidy", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create log directory: %w", err)
	}
	
	now := time.Now()
	filename := fmt.Sprintf("%s.%03d.json",
		now.Format("2006-01-02_150405"),
		now.Nanosecond()/1000000)
	
	return filepath.Join(logDir, filename), nil
}

func WriteSession(session *LogSession) error {
	if session == nil {
		return nil
	}
	
	logPath, err := GetLogPath()
	if err != nil {
		return fmt.Errorf("failed to get log path: %w", err)
	}
	
	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}
	
	if err := os.WriteFile(logPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write log file: %w", err)
	}
	
	return nil
}

func ReadSession(logPath string) (*LogSession, error) {
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}
	
	var session LogSession
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}
	
	return &session, nil
}

func ReadSessions(limit int) ([]*LogSession, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	
	logDir := filepath.Join(homeDir, ".title-tidy", "logs")
	
	// Check if log directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return []*LogSession{}, nil
	}
	
	files, err := filepath.Glob(filepath.Join(logDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list log files: %w", err)
	}
	
	// Sort files by name (which includes timestamp) in descending order
	sort.Sort(sort.Reverse(sort.StringSlice(files)))
	
	// Apply limit
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	
	sessions := make([]*LogSession, 0, len(files))
	for _, file := range files {
		session, err := ReadSession(file)
		if err != nil {
			// Skip corrupted files
			continue
		}
		sessions = append(sessions, session)
	}
	
	return sessions, nil
}

// cleanupOldLogsUnsafe performs cleanup without acquiring mutex (assumes caller holds it)
func cleanupOldLogsUnsafe(retentionDays int) error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	
	logDir := filepath.Join(homeDir, ".title-tidy", "logs")
	
	// Check if log directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil
	}
	
	files, err := filepath.Glob(filepath.Join(logDir, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list log files: %w", err)
	}
	
	cutoff := time.Now().AddDate(0, 0, -retentionDays)
	
	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		
		if info.ModTime().Before(cutoff) {
			if err := os.Remove(file); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to remove old log file %s: %v\n", file, err)
				continue
			}
		}
	}
	
	return nil
}