package core

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/Digital-Shane/title-tidy/internal/log"
	"github.com/Digital-Shane/treeview"
)

// RenameRegular renames a node; returns true only when an actual filesystem rename occurred.
func RenameRegular(node *treeview.Node[treeview.FileInfo], mm *MediaMeta) (bool, error) {
	oldPath := node.Data().Path
	newName, err := sanitizeFilename(mm.NewName)
	if err != nil {
		log.LogRename(oldPath, "", false, err)
		return false, mm.Fail(err)
	}
	if newName != mm.NewName {
		mm.NewName = newName
	}

	newPath := filepath.Join(filepath.Dir(oldPath), newName)
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
func CreateVirtualDir(node *treeview.Node[treeview.FileInfo], mm *MediaMeta) (int, []error) {
	successes := 0
	errs := []error{}

	dirName, err := sanitizeFilename(mm.NewName)
	if err != nil {
		log.LogCreateDir(mm.NewName, false, err)
		errs = append(errs, mm.Fail(err))
		return successes, errs
	}
	if dirName != mm.NewName {
		mm.NewName = dirName
	}

	dirPath := filepath.Join(".", dirName)
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
		cm := GetMeta(child)
		if cm == nil {
			continue
		}

		// Use NewName if set, otherwise keep original name
		childName := cm.NewName
		if childName == "" {
			childName = child.Name()
		}
		childName, err := sanitizeFilename(childName)
		if err != nil {
			log.LogRename(child.Data().Path, "", false, err)
			errs = append(errs, fmt.Errorf("%s: %w", child.Name(), cm.Fail(err)))
			continue
		}
		if childName != cm.NewName {
			cm.NewName = childName
		}

		oldChildPath := child.Data().Path
		newChildPath := filepath.Join(dirPath, childName)
		if err := os.Rename(oldChildPath, newChildPath); err != nil {
			log.LogRename(oldChildPath, newChildPath, false, err)
			errs = append(errs, fmt.Errorf("%s -> %s: %w", child.Name(), childName, cm.Fail(err)))
			continue
		}
		log.LogRename(oldChildPath, newChildPath, true, nil)
		successes++
		cm.Success()
		child.Data().Path = newChildPath
	}
	return successes, errs
}

// LinkRegular creates a hard link from the source node to the destination path.
// Returns true only when an actual filesystem link was created.
func LinkRegular(node *treeview.Node[treeview.FileInfo], mm *MediaMeta) (bool, error) {
	srcPath := node.Data().Path
	destPath := mm.DestinationPath

	if destPath == "" {
		err := fmt.Errorf("no destination path specified")
		log.LogLink(srcPath, destPath, false, err)
		return false, mm.Fail(err)
	}

	sanitizedPath, err := sanitizePath(destPath)
	if err != nil {
		log.LogLink(srcPath, destPath, false, err)
		return false, mm.Fail(err)
	}
	if sanitizedPath != destPath {
		destPath = sanitizedPath
		mm.DestinationPath = sanitizedPath
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
func LinkVirtualDir(node *treeview.Node[treeview.FileInfo], mm *MediaMeta, linkPath string) (int, []error) {
	successes := 0
	errs := []error{}

	// Create directory in the destination
	dirName, err := sanitizeFilename(mm.NewName)
	if err != nil {
		log.LogCreateDir(mm.NewName, false, err)
		errs = append(errs, mm.Fail(err))
		return successes, errs
	}
	if dirName != mm.NewName {
		mm.NewName = dirName
	}

	dirPath := filepath.Join(linkPath, dirName)
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
		cm := GetMeta(child)
		if cm == nil {
			continue
		}

		// Use NewName if set, otherwise keep original name
		childName := cm.NewName
		if childName == "" {
			childName = child.Name()
		}
		childName, err := sanitizeFilename(childName)
		if err != nil {
			log.LogLink(child.Data().Path, "", false, err)
			errs = append(errs, fmt.Errorf("%s: %w", child.Name(), cm.Fail(err)))
			continue
		}
		if childName != cm.NewName {
			cm.NewName = childName
		}

		srcPath := child.Data().Path
		destPath := filepath.Join(dirPath, childName)

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
				errs = append(errs, fmt.Errorf("%s -> %s: failed to create hard link (possibly cross-filesystem or unsupported): %w", child.Name(), childName, err))
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

// DeleteMarkedNode removes the file on disk for a node marked for deletion.
func DeleteMarkedNode(node *treeview.Node[treeview.FileInfo], mm *MediaMeta) error {
	filePath := node.Data().Path
	if err := os.Remove(filePath); err != nil {
		log.LogDelete(filePath, false, err)
		mm.Fail(err)
		return err
	}
	log.LogDelete(filePath, true, nil)
	mm.Success()
	return nil
}

// EnsureDestinationDir makes sure the destination directory exists for link mode operations.
func EnsureDestinationDir(path string, mm *MediaMeta) error {
	sanitizedPath, err := sanitizePath(path)
	if err != nil {
		log.LogCreateDir(path, false, err)
		mm.Fail(err)
		return err
	}
	if sanitizedPath != path {
		path = sanitizedPath
		mm.DestinationPath = sanitizedPath
	}

	if err := os.MkdirAll(path, 0755); err != nil {
		log.LogCreateDir(path, false, err)
		mm.Fail(err)
		return err
	}
	log.LogCreateDir(path, true, nil)
	mm.Success()
	return nil
}
