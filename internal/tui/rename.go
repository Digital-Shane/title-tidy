package tui

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/treeview"
	tea "github.com/charmbracelet/bubbletea"
)

// RenameCompleteMsg is emitted once performRenames finishes walking the tree.
type RenameCompleteMsg struct{ successCount, errorCount int }

// SuccessCount returns the number of successful renames
func (r RenameCompleteMsg) SuccessCount() int { return r.successCount }

// ErrorCount returns the number of errors during renames
func (r RenameCompleteMsg) ErrorCount() int { return r.errorCount }

// internal progress message for streaming rename updates
type renameProgressMsg struct{}

// prepareRenameProgress counts total operations (renames, deletions, virtual dir creations)
func (m *RenameModel) prepareRenameProgress() {
	// Count operations without storing them to save memory
	m.virtualDirCount = 0
	m.deletionCount = 0
	m.renameCount = 0

	// Single pass to count all operation types
	for info, _ := range m.Tree.All(context.Background()) {
		n := info.Node
		mm := core.GetMeta(n)
		if mm == nil {
			continue
		}
		if mm.MarkedForDeletion {
			m.deletionCount++
			continue
		}
		if mm.NeedsDirectory && mm.IsVirtual {
			m.virtualDirCount++
			continue
		}
		// Skip children of virtual dirs as they're handled with their parent
		if parent := n.Parent(); parent != nil {
			if pm := core.GetMeta(parent); pm != nil && pm.IsVirtual {
				continue
			}
		}
		if mm.NewName != "" && mm.NewName != n.Name() {
			m.renameCount++
		}
	}

	// Total operations: virtual dirs + deletions + regular renames
	m.totalRenameOps = m.virtualDirCount + m.deletionCount + m.renameCount
	m.completedOps = 0
	m.currentOpIndex = 0
}

// RenameRegular renames a node; returns true only when an actual filesystem rename occurred.
func RenameRegular(node *treeview.Node[treeview.FileInfo], mm *core.MediaMeta) (bool, error) {
	oldPath := node.Data().Path
	newPath := filepath.Join(filepath.Dir(oldPath), mm.NewName)
	if oldPath == newPath {
		return false, nil
	}
	if _, err := os.Stat(newPath); err == nil {
		return false, mm.Fail(fmt.Errorf("destination already exists"))
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return false, mm.Fail(err)
	}
	mm.Success()
	node.Data().Path = newPath
	return true, nil
}

// calculateTargetPath computes the target path for a link, preserving directory structure
func calculateTargetPath(sourcePath string, targetRoot string, newName string) (string, error) {
	// Get absolute source path
	absSource, err := filepath.Abs(sourcePath)
	if err != nil {
		return "", err
	}
	
	// Get current working directory to calculate relative path
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	
	// Calculate relative path from cwd
	relPath, err := filepath.Rel(cwd, absSource)
	if err != nil {
		return "", err
	}
	
	// Preserve directory structure in target
	targetDir := targetRoot
	if relDir := filepath.Dir(relPath); relDir != "." {
		targetDir = filepath.Join(targetRoot, relDir)
	}
	
	return filepath.Join(targetDir, newName), nil
}

// LinkRegular creates a link to a node; returns true only when a link was successfully created.
// It tries hard linking first, then falls back to soft linking if that fails.
func LinkRegular(node *treeview.Node[treeview.FileInfo], mm *core.MediaMeta, linkMode core.LinkMode, linkTarget string) (bool, error) {
	oldPath := node.Data().Path
	
	// Determine target path
	var targetPath string
	var err error
	if linkTarget != "" {
		// Link to different directory with structure preservation
		targetPath, err = calculateTargetPath(oldPath, linkTarget, mm.NewName)
		if err != nil {
			return false, mm.Fail(err)
		}
		
		// Ensure target directory exists
		targetDir := filepath.Dir(targetPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			return false, mm.Fail(fmt.Errorf("failed to create target directory: %w", err))
		}
	} else {
		// Link in current directory (alongside original)
		targetPath = filepath.Join(filepath.Dir(oldPath), mm.NewName)
	}
	
	// Check if we're trying to link to the same file
	if oldPath == targetPath {
		return false, nil
	}
	
	// Check if target already exists
	if _, err := os.Stat(targetPath); err == nil {
		return false, mm.Fail(fmt.Errorf("destination already exists"))
	}
	
	// Get absolute path for source (needed for symlinks)
	absOldPath, err := filepath.Abs(oldPath)
	if err != nil {
		return false, mm.Fail(err)
	}
	
	// Try hard link first if in auto or hard mode
	if linkMode == core.LinkModeAuto || linkMode == core.LinkModeHard {
		if err := os.Link(absOldPath, targetPath); err == nil {
			mm.Success()
			// Update node path to reflect the link location
			node.Data().Path = targetPath
			return true, nil
		} else if linkMode == core.LinkModeHard {
			// Hard link required but failed
			return false, mm.Fail(fmt.Errorf("hard link failed: %w", err))
		}
		// Auto mode: fall through to try soft link
	}
	
	// Create soft link
	if err := os.Symlink(absOldPath, targetPath); err != nil {
		return false, mm.Fail(fmt.Errorf("soft link failed: %w", err))
	}
	
	mm.Success()
	// Update node path to reflect the link location
	node.Data().Path = targetPath
	return true, nil
}

// CreateVirtualDir materializes a virtual movie directory then renames its children beneath it.
//
// Returns a count of successful operations (directory creation + child renames), and contextual errors
func CreateVirtualDir(node *treeview.Node[treeview.FileInfo], mm *core.MediaMeta) (int, []error) {
	successes := 0
	errs := []error{}

	dirPath := filepath.Join(".", mm.NewName)
	if err := os.Mkdir(dirPath, 0755); err != nil {
		errs = append(errs, fmt.Errorf("create %s: %w", mm.NewName, mm.Fail(err)))
		return successes, errs
	}

	// Directory created successfully
	successes++
	mm.Success()
	node.Data().Path = dirPath

	// Rename children into the new directory
	for _, child := range node.Children() {
		cm := core.GetMeta(child)
		if cm == nil || cm.NewName == "" {
			continue
		}
		oldChildPath := child.Data().Path
		newChildPath := filepath.Join(dirPath, cm.NewName)
		if err := os.Rename(oldChildPath, newChildPath); err != nil {
			errs = append(errs, fmt.Errorf("%s -> %s: %w", child.Name(), cm.NewName, cm.Fail(err)))
			continue
		}
		successes++
		cm.Success()
		child.Data().Path = newChildPath
	}
	return successes, errs
}

// CreateVirtualDirWithLinks materializes a virtual movie directory then links its children into it.
//
// Returns a count of successful operations (directory creation + child links), and contextual errors
func CreateVirtualDirWithLinks(node *treeview.Node[treeview.FileInfo], mm *core.MediaMeta, linkMode core.LinkMode, linkTarget string) (int, []error) {
	successes := 0
	errs := []error{}

	// Determine where to create the directory
	var dirPath string
	if linkTarget != "" {
		// Create directory in target location
		// Calculate relative path for the virtual directory
		relPath := mm.NewName
		dirPath = filepath.Join(linkTarget, relPath)
	} else {
		// Create directory in current location
		dirPath = filepath.Join(".", mm.NewName)
	}

	// Create the directory
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		errs = append(errs, fmt.Errorf("create %s: %w", dirPath, mm.Fail(err)))
		return successes, errs
	}

	// Directory created successfully
	successes++
	mm.Success()
	node.Data().Path = dirPath

	// Link children into the new directory
	for _, child := range node.Children() {
		cm := core.GetMeta(child)
		if cm == nil || cm.NewName == "" {
			continue
		}
		
		// Get absolute path of source file
		absOldPath, err := filepath.Abs(child.Data().Path)
		if err != nil {
			errs = append(errs, fmt.Errorf("abs path %s: %w", child.Name(), cm.Fail(err)))
			continue
		}
		
		targetPath := filepath.Join(dirPath, cm.NewName)
		
		// Try hard link first if in auto or hard mode
		linked := false
		if linkMode == core.LinkModeAuto || linkMode == core.LinkModeHard {
			if err := os.Link(absOldPath, targetPath); err == nil {
				linked = true
			} else if linkMode == core.LinkModeHard {
				// Hard link required but failed
				errs = append(errs, fmt.Errorf("%s hard link: %w", child.Name(), cm.Fail(err)))
				continue
			}
		}
		
		// Fall back to soft link if not linked yet
		if !linked {
			if err := os.Symlink(absOldPath, targetPath); err != nil {
				errs = append(errs, fmt.Errorf("%s soft link: %w", child.Name(), cm.Fail(err)))
				continue
			}
		}
		
		successes++
		cm.Success()
		child.Data().Path = targetPath
	}
	return successes, errs
}

// PerformRenames walks the tree bottom‑up executing pending rename operations.
// It skips children of virtual directories (handled by the virtual parent) and
// aggregates success / error counts into a renameCompleteMsg.
//
// This function is designed to be called repeatedly by Bubble Tea, processing one
// operation at a time and yielding control back to the UI between operations.
// This allows for progress updates and maintains UI responsiveness during long
// rename operations.
func (m *RenameModel) PerformRenames() tea.Cmd {
	return func() tea.Msg {
		// Check if all operations have been completed
		if m.completedOps >= m.totalRenameOps {
			return RenameCompleteMsg{successCount: m.successCount, errorCount: m.errorCount}
		}
		currentCount := 0

		// Phase 1: Virtual directories
		// These are processed first because child files will be moved into them
		if m.currentOpIndex < m.virtualDirCount {
			// Iterate through tree to find the nth virtual directory
			for info := range m.Tree.All(context.Background()) {
				node := info.Node
				mm := core.GetMeta(node)
				if mm != nil && mm.NeedsDirectory && mm.IsVirtual {
					// Found a virtual directory
					// check if it's the one we need to process
					if currentCount == m.currentOpIndex {
						// Create the directory and handle children based on link mode
						var s int
						var errs []error
						if m.LinkMode != core.LinkModeNone {
							s, errs = CreateVirtualDirWithLinks(node, mm, m.LinkMode, m.LinkTarget)
						} else {
							s, errs = CreateVirtualDir(node, mm)
						}
						m.successCount += s
						m.errorCount += len(errs)
						m.completedOps++
						m.currentOpIndex++
						break // Yield control back to UI
					}
					currentCount++
				}
			}
		} else if m.currentOpIndex < m.virtualDirCount+m.deletionCount {
			// Phase 2: Deletions (NFO files, images, etc. marked for removal)
			// Calculate which deletion we're looking for in this phase
			targetIndex := m.currentOpIndex - m.virtualDirCount
			for info := range m.Tree.All(context.Background()) {
				node := info.Node
				mm := core.GetMeta(node)
				if mm != nil && mm.MarkedForDeletion {
					// Found a file to delete
					// check if it's the one we need to process
					if currentCount == targetIndex {
						// Attempt to delete the file
						if err := os.Remove(node.Data().Path); err != nil {
							mm.Fail(err)
							m.errorCount++
						} else {
							mm.Success()
							m.successCount++
						}
						m.completedOps++
						m.currentOpIndex++
						break // Yield control back to UI
					}
					currentCount++
				}
			}
		} else {
			// Phase 3: Regular renames (standard file/folder renames)
			// Process bottom-up so child renames happen before parent renames
			targetIndex := m.currentOpIndex - m.virtualDirCount - m.deletionCount
			for info := range m.Tree.AllBottomUp(context.Background()) {
				node := info.Node
				mm := core.GetMeta(node)
				if mm == nil {
					continue
				}
				// Skip operations already handled in previous phases
				if mm.MarkedForDeletion || (mm.NeedsDirectory && mm.IsVirtual) {
					continue
				}
				// Skip children of virtual dirs (they're moved by their parent's CreateVirtualDir)
				if parent := node.Parent(); parent != nil {
					if pm := core.GetMeta(parent); pm != nil && pm.IsVirtual {
						continue
					}
				}
				// Only process nodes that actually need renaming
				if mm.NewName != "" && mm.NewName != node.Name() {
					// Found a file to rename or link
					// check if it's the one we need to process
					if currentCount == targetIndex {
						// Perform the filesystem operation based on link mode
						var operated bool
						var err error
						if m.LinkMode != core.LinkModeNone {
							operated, err = LinkRegular(node, mm, m.LinkMode, m.LinkTarget)
						} else {
							operated, err = RenameRegular(node, mm)
						}
						
						if err != nil {
							m.errorCount++
						} else if operated {
							m.successCount++
						}
						m.completedOps++
						m.currentOpIndex++
						break // Yield control back to UI
					}
					currentCount++
				}
			}
		}

		// Check again if all operations are now complete
		if m.completedOps >= m.totalRenameOps {
			return RenameCompleteMsg{successCount: m.successCount, errorCount: m.errorCount}
		}

		// Return progress message to continue processing in next Bubble Tea cycle
		return renameProgressMsg{}
	}
}
