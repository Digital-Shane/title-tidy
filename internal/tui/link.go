package tui

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/core"
	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/treeview"
)

// LinkRegular creates a hard link from the source node to the destination path.
// Returns true only when an actual filesystem link was created.
func LinkRegular(node *treeview.Node[treeview.FileInfo], mm *core.MediaMeta) (bool, error) {
	srcPath := node.Data().Path
	destPath := mm.DestinationPath

	if destPath == "" {
		err := fmt.Errorf("no destination path specified")
		log.LogLink(srcPath, destPath, false, err)
		return false, mm.Fail(err)
	}

	// Create parent directory if it doesn't exist
	destDir := filepath.Dir(destPath)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		err := fmt.Errorf("failed to create directory %s: %w", destDir, err)
		log.LogLink(srcPath, destPath, false, err)
		return false, mm.Fail(err)
	}

	// Check if destination already exists
	if _, err := os.Stat(destPath); err == nil {
		// File already exists - treat as success for incremental linking
		log.LogLink(srcPath, destPath, true, nil)
		mm.Success()
		return false, nil // Return false because no new link was created
	}

	// Create the hard link
	if err := os.Link(srcPath, destPath); err != nil {
		if os.IsExist(err) {
			// File was created between our check and link attempt - treat as success
			log.LogLink(srcPath, destPath, true, nil)
			mm.Success()
			return false, nil
		}
		log.LogLink(srcPath, destPath, false, err)
		return false, mm.Fail(fmt.Errorf("failed to create hard link (possibly cross-filesystem or unsupported): %w", err))
	}

	log.LogLink(srcPath, destPath, true, nil)
	mm.Success()
	return true, nil
}

// LinkVirtualDir creates a virtual movie directory in the destination and links its children into it.
// Returns a count of successful operations (directory creation + child links), and contextual errors
func LinkVirtualDir(node *treeview.Node[treeview.FileInfo], mm *core.MediaMeta, linkPath string) (int, []error) {
	successes := 0
	errs := []error{}

	// Create directory in the destination
	dirPath := filepath.Join(linkPath, mm.NewName)
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		log.LogCreateDir(dirPath, false, err)
		errs = append(errs, fmt.Errorf("create %s: %w", mm.NewName, mm.Fail(err)))
		return successes, errs
	}

	// Directory created successfully (MkdirAll is idempotent, so existing dirs are OK)
	log.LogCreateDir(dirPath, true, nil)
	successes++
	mm.Success()

	// Link children into the new directory
	for _, child := range node.Children() {
		cm := core.GetMeta(child)
		if cm == nil || cm.NewName == "" {
			continue
		}

		srcPath := child.Data().Path
		destPath := filepath.Join(dirPath, cm.NewName)

		// Check if destination already exists
		if _, err := os.Stat(destPath); err == nil {
			// File already exists - treat as success for incremental linking
			log.LogLink(srcPath, destPath, true, nil)
			successes++
			cm.Success()
			cm.DestinationPath = destPath
			continue
		}

		if err := os.Link(srcPath, destPath); err != nil {
			if os.IsExist(err) {
				// File was created between our check and link attempt - treat as success
				log.LogLink(srcPath, destPath, true, nil)
				successes++
				cm.Success()
				cm.DestinationPath = destPath
			} else {
				log.LogLink(srcPath, destPath, false, err)
				errs = append(errs, fmt.Errorf("%s -> %s: failed to create hard link (possibly cross-filesystem or unsupported): %w", child.Name(), cm.NewName, err))
				cm.Fail(fmt.Errorf("failed to create hard link (possibly cross-filesystem or unsupported): %w", err))
			}
			continue
		}

		log.LogLink(srcPath, destPath, true, nil)
		successes++
		cm.Success()
		cm.DestinationPath = destPath
	}

	return successes, errs
}
