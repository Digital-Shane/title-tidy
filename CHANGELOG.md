# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v1.3.2] - 2025-08-21
### Added
- Github actions for building and pushing images to docker hub.

## [v1.3.1] - 2025-08-20
### Fixed
- TUI distortion when title-tidy is used over ssh
### Added
- Github actions for build validation and test coverage.
- More unit tests to meet new testing requirements.

## [v1.3.0] - 2025-08-20
### Added
- Delete key support to remove tree nodes and cancel rename operations.
  - Use `delete` or `d` key to remove focused nodes from the tree.
  - Removes the node and all child operations from rename processing.
  - Focus automatically moves up one position after deletion for smooth navigation.
### Updated
- All demo gifs.

## [v1.2.0] - 2025-08-19
### Added
- Progress bar during file indexing.
  - Is fairly accurate and quick by tracking root level nodes processed over pre indexing the whole file tree.
- Progress bar to track the status of delete, rename, and create directory operations.
### Updated
- Go Dependencies.
- Stat panel to hug the right side of the terminal.
- Run `go fmt ./...` on the project.
- All demo gifs.

## [v1.1.1] - 2025-08-17
### Updated
- treeview dependency to v1.5.1 to vastly improve render performance for large trees.

## [v1.1.0] - 2025-08-16
### Added
- Added new option --no-nfo which deletes nfo files as part of the rename process
- Added new option --no-img which deletes image files as part of the rename process
- Updated show demo to include new flag functionality

## [v1.0.1] - 2025-08-16
### Fixed
- Extra vertical bar next to stat panel.
- Extra left padding for root level nodes.
- Page up and Page down support.
- Scroll wheel support. 

## [v1.0.0] - 2025-08-12
### Released
- My utility for quickly renaming acquired media. 😁