package log

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type UndoResult struct {
	Operation OperationLog
	Success   bool
	Error     error
}

func UndoOperation(op OperationLog) UndoResult {
	result := UndoResult{
		Operation: op,
		Success:   false,
	}
	
	switch op.Type {
	case OpRename:
		// Reverse a rename operation: rename back to original
		if op.DestPath == "" {
			result.Error = fmt.Errorf("cannot undo rename: destination path missing")
			return result
		}
		
		// Check if the destination file exists (the renamed file)
		if _, err := os.Stat(op.DestPath); os.IsNotExist(err) {
			result.Error = fmt.Errorf("cannot undo rename: file %s not found", op.DestPath)
			return result
		}
		
		// Check if reverting would overwrite an existing file
		if _, err := os.Stat(op.SourcePath); err == nil {
			result.Error = fmt.Errorf("cannot undo rename: original path %s already exists", op.SourcePath)
			return result
		}
		
		// Perform the reverse rename
		if err := os.Rename(op.DestPath, op.SourcePath); err != nil {
			result.Error = fmt.Errorf("failed to rename %s back to %s: %w", op.DestPath, op.SourcePath, err)
			return result
		}
		
		result.Success = true
		
	case OpLink:
		// Reverse a link operation: remove the link
		if op.DestPath == "" {
			result.Error = fmt.Errorf("cannot undo link: destination path missing")
			return result
		}
		
		// Verify the link exists
		info, err := os.Lstat(op.DestPath)
		if os.IsNotExist(err) {
			// Link already removed, consider it successful
			result.Success = true
			return result
		}
		
		// For safety, verify it's actually a link to the expected source
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink, check if it points to our source
			target, err := os.Readlink(op.DestPath)
			if err == nil && target != op.SourcePath {
				result.Error = fmt.Errorf("link target mismatch: expected %s, got %s", op.SourcePath, target)
				return result
			}
		}
		
		// Remove the link
		if err := os.Remove(op.DestPath); err != nil {
			result.Error = fmt.Errorf("failed to remove link %s: %w", op.DestPath, err)
			return result
		}
		
		result.Success = true
		
	case OpCreateDir:
		// Reverse a directory creation: remove if empty
		if op.DestPath == "" && op.SourcePath != "" {
			// For directory creation, the path might be in SourcePath
			op.DestPath = op.SourcePath
		}
		
		if op.DestPath == "" {
			result.Error = fmt.Errorf("cannot undo directory creation: path missing")
			return result
		}
		
		// Check if directory exists
		info, err := os.Stat(op.DestPath)
		if os.IsNotExist(err) {
			// Directory already removed, consider it successful
			result.Success = true
			return result
		}
		
		if !info.IsDir() {
			result.Error = fmt.Errorf("path %s is not a directory", op.DestPath)
			return result
		}
		
		// Check if directory is empty
		entries, err := os.ReadDir(op.DestPath)
		if err != nil {
			result.Error = fmt.Errorf("failed to read directory %s: %w", op.DestPath, err)
			return result
		}
		
		if len(entries) > 0 {
			result.Error = fmt.Errorf("cannot remove directory %s: not empty", op.DestPath)
			return result
		}
		
		// Remove the empty directory
		if err := os.Remove(op.DestPath); err != nil {
			result.Error = fmt.Errorf("failed to remove directory %s: %w", op.DestPath, err)
			return result
		}
		
		result.Success = true
		
	case OpDelete:
		// For now, we can't restore deleted files
		// Future enhancement: check trash/recycle bin
		result.Error = fmt.Errorf("cannot undo delete operations (file restoration not yet implemented)")
		
	default:
		result.Error = fmt.Errorf("unknown operation type: %s", op.Type)
	}
	
	return result
}

func UndoSession(session *LogSession) (successful int, failed int, errors []error) {
	// Process operations in reverse order
	for i := len(session.Operations) - 1; i >= 0; i-- {
		op := session.Operations[i]
		
		// Only undo successful operations
		if !op.Success {
			continue
		}
		
		result := UndoOperation(op)
		if result.Success {
			successful++
		} else {
			failed++
			if result.Error != nil {
				errors = append(errors, result.Error)
			}
		}
	}
	
	return successful, failed, errors
}

func FindLatestSession() (*LogSession, string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	logDir := filepath.Join(homeDir, ".title-tidy", "logs")
	
	// Check if log directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("no log directory found")
	}
	
	// Get the most recent session
	sessions, err := ReadSessions(1)
	if err != nil {
		return nil, "", fmt.Errorf("failed to read sessions: %w", err)
	}
	
	if len(sessions) == 0 {
		return nil, "", fmt.Errorf("no sessions found")
	}
	
	// Get the file path for the latest session
	files, err := filepath.Glob(filepath.Join(logDir, "*.json"))
	if err != nil || len(files) == 0 {
		return nil, "", fmt.Errorf("no log files found")
	}
	
	// Files are already sorted, take the latest
	latestFile := files[len(files)-1]
	
	return sessions[0], latestFile, nil
}

type SessionSummary struct {
	Session      *LogSession
	FilePath     string
	RelativeTime string
	Icon         string
}

func GetSessionSummaries() ([]SessionSummary, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	
	logDir := filepath.Join(homeDir, ".title-tidy", "logs")
	
	// Check if log directory exists
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return []SessionSummary{}, nil
	}
	
	files, err := filepath.Glob(filepath.Join(logDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list log files: %w", err)
	}
	
	// Sort files by name (newest first)
	for i := 0; i < len(files)/2; i++ {
		files[i], files[len(files)-1-i] = files[len(files)-1-i], files[i]
	}
	
	summaries := make([]SessionSummary, 0, len(files))
	for _, file := range files {
		session, err := ReadSession(file)
		if err != nil {
			continue
		}
		
		summary := SessionSummary{
			Session:      session,
			FilePath:     file,
			RelativeTime: formatRelativeTime(session.Metadata.Timestamp),
			Icon:         getCommandIcon(session.Metadata.CommandArgs),
		}
		summaries = append(summaries, summary)
	}
	
	return summaries, nil
}

func formatRelativeTime(t time.Time) string {
    duration := time.Since(t)
    switch {
    case duration < time.Minute:
        return "just now"
    case duration < time.Hour:
        mins := int(duration.Minutes())
        return fmt.Sprintf("%d minute%s ago", mins, plural(mins))
    case duration < 24*time.Hour:
        hours := int(duration.Hours())
        return fmt.Sprintf("%d hour%s ago", hours, plural(hours))
    case duration < 7*24*time.Hour:
        days := int(duration.Hours() / 24)
		return fmt.Sprintf("%d day%s ago", days, plural(days))
    default:
        return t.Format("Jan 2, 2006")
    }
}

func plural(n int) string {
    if n == 1 {
        return ""
    }
    return "s"
}


func getCommandIcon(args []string) string {
	if len(args) == 0 {
		return "❓"
	}
	
	command := args[0]
	switch command {
	case "shows":
		return "📺"
	case "seasons":
		return "📁"
	case "episodes":
		return "🎬"
	case "movies":
		return "🎬"
	default:
		return "📝"
	}
}