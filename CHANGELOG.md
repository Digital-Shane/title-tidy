# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## Unreleased

## [v1.8.3] - 2025-09-07
### Added
* New release pipeline for those that want to download executables. 

## [v1.8.2] - 2025-09-04
### Fixed
* channel race condition during metadata parsing

## [v1.8.1] - 2025-09-04
### Fixed
* metadata progress loop confusing episodes for movies when running `title-tidy episodes`.

## [v1.8.0] - 2025-09-04
### Added
* new `--no-sample` command that deletes sample media files.

## [v1.7.4] - 2025-09-03
### Fixed
* powershell adding null unicode characters at the end of config strings.
* Use SSH icon set in powershell because fancy icons are not supported.

## [v1.7.3] - 2025-09-03
### Updated
* `treeview` dependency to `v1.8.1` to fix inode implementation on windows.

## [v1.7.2] - 2025-09-02
### Updated
* Respect the max workers config to a greater degree

## [v1.7.1] - 2025-09-02
### Updated
* Allow configuring the amount of TMDB workers in the config.
* Increase default workers to 20 from 6.

## [v1.7.0] - 2025-09-02
### Updated
- README.md to include tmdb config settings.
- treeview library to `v1.8.0` to fix TUI distortion caused by viewport line wrapping.
### Added
- Ability to configure TMDB settings.
- TMDB provider for pulling metadata.
- Template system extended to use metadata providers.
- Metadata progress bar that shows while fetching TMDB data before the rename preview.
### Fixed
- Remove deprecated bubble tea api usage.
- go fmt

## [v1.6.0] - 2025-08-26
### Added
- Logging title-tidy operations to a file.
- Configurable log retention.
- Ability to undo a rename/relink instantly after it is performed.
- `title-tidy undo` to undo rename/relink after the title tidy session has closed.
- viewport support to stat panels to fix TUI distortion when terminal is too short.
- Allow configuring logging parameters via the config UI.
- New demo to show undo command. 
### Updated
- Config demo gif to show new section. 
- Readme to detail logging and undo functionality.

## [v1.5.1] - 2025-08-26
### Fixed
- TUI distortion when the file tree exceeds the height of the terminal.
### Updated
- Github workflows to only run when go files or the dockerfile changes.

## [v1.5.0] - 2025-08-25
### Added
- `--link` support to hard link existing files to a new destination instead of renaming files in place.

## [v1.4.0] - 2025-08-24
### Added
- New UI to allow customizing the name template for all supported media types.
- New Logo in the README.md

## [v1.3.3] - 2025-08-22
### Updated
- Dockerfile to use a full alpine image for shell support

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