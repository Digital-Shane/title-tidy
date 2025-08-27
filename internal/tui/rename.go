package tui

import (
	"context"
	"fmt"
	"iter"
	"os"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/log"
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
			if !m.IsLinkMode { // Don't count deletions in link mode
				m.deletionCount++
			}
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
		if m.IsLinkMode {
			// In link mode, count nodes with destination paths
			if mm.DestinationPath != "" {
				m.renameCount++
			}
		} else {
			// In rename mode, count nodes that need renaming
			if mm.NewName != "" && mm.NewName != n.Name() {
				m.renameCount++
			}
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
		err := fmt.Errorf("destination already exists")
		log.LogRename(oldPath, newPath, false, err)
		return false, mm.Fail(err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		log.LogRename(oldPath, newPath, false, err)
		return false, mm.Fail(err)
	}
	log.LogRename(oldPath, newPath, true, nil)
	mm.Success()
	node.Data().Path = newPath
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
		log.LogCreateDir(dirPath, false, err)
		errs = append(errs, fmt.Errorf("create %s: %w", mm.NewName, mm.Fail(err)))
		return successes, errs
	}

	// Directory created successfully
	log.LogCreateDir(dirPath, true, nil)
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
			log.LogRename(oldChildPath, newChildPath, false, err)
			errs = append(errs, fmt.Errorf("%s -> %s: %w", child.Name(), cm.NewName, cm.Fail(err)))
			continue
		}
		log.LogRename(oldChildPath, newChildPath, true, nil)
		successes++
		cm.Success()
		child.Data().Path = newChildPath
	}
	return successes, errs
}

// PerformRenames walks the tree executing pending rename or link operations.
// In rename mode, it uses bottom-up traversal. In link mode, it uses breadth-first
// to ensure parent directories exist before linking children.
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
		// These are processed first because child files will be moved/linked into them
		if m.currentOpIndex < m.virtualDirCount {
			// Iterate through tree to find the nth virtual directory
			for info := range m.Tree.All(context.Background()) {
				node := info.Node
				mm := core.GetMeta(node)
				if mm != nil && mm.NeedsDirectory && mm.IsVirtual {
					// Found a virtual directory
					// check if it's the one we need to process
					if currentCount == m.currentOpIndex {
						// Create the directory and move/link its children into it
						var s int
						var errs []error
						if m.IsLinkMode {
							s, errs = LinkVirtualDir(node, mm, m.LinkPath)
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
			// Skip deletions entirely in link mode - we never delete when linking
			if !m.IsLinkMode {
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
							filePath := node.Data().Path
							if err := os.Remove(filePath); err != nil {
								log.LogDelete(filePath, false, err)
								mm.Fail(err)
								m.errorCount++
							} else {
								log.LogDelete(filePath, true, nil)
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
				// In link mode, skip this phase entirely
				m.currentOpIndex += m.deletionCount
				m.completedOps += m.deletionCount
			}
		} else {
			// Phase 3: Regular renames/links (standard file/folder operations)
			// In rename mode: use bottom-up so child renames happen before parent renames
			// In link mode: use breadth-first so parent dirs are created before children
			targetIndex := m.currentOpIndex - m.virtualDirCount - m.deletionCount

			var iterator iter.Seq2[treeview.NodeInfo[treeview.FileInfo], error]
			if m.IsLinkMode {
				iterator = m.Tree.BreadthFirst(context.Background())
			} else {
				iterator = m.Tree.AllBottomUp(context.Background())
			}

			for info := range iterator {
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
				// Process differently based on mode
				if m.IsLinkMode {
					// In link mode, check if we need to create a directory or link a file
					if mm.DestinationPath != "" {
						if currentCount == targetIndex {
							if node.Data().IsDir() {
								// Create directory in destination
								if err := os.MkdirAll(mm.DestinationPath, 0755); err != nil {
									log.LogCreateDir(mm.DestinationPath, false, err)
									mm.Fail(err)
									m.errorCount++
								} else {
									log.LogCreateDir(mm.DestinationPath, true, nil)
									mm.Success()
									m.successCount++
								}
							} else {
								// Link file to destination
								if linked, err := LinkRegular(node, mm); err != nil {
									m.errorCount++
								} else if linked {
									m.successCount++
								}
							}
							m.completedOps++
							m.currentOpIndex++
							break // Yield control back to UI
						}
						currentCount++
					}
				} else {
					// Only process nodes that actually need renaming
					if mm.NewName != "" && mm.NewName != node.Name() {
						// Found a file to rename
						// check if it's the one we need to process
						if currentCount == targetIndex {
							// Perform the filesystem rename operation
							if renamed, err := RenameRegular(node, mm); err != nil {
								m.errorCount++
							} else if renamed {
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
		}

		// Check again if all operations are now complete
		if m.completedOps >= m.totalRenameOps {
			return RenameCompleteMsg{successCount: m.successCount, errorCount: m.errorCount}
		}

		// Return progress message to continue processing in next Bubble Tea cycle
		return renameProgressMsg{}
	}
}
