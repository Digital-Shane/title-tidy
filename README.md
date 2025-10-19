<div align="center">
  <img src="logo.png" alt="Title Tidy Logo" height="300">
  <p style="color: #888; font-weight: bold; font-style: italic; font-size: 1.5em;">The free FileBot alternative</p>
</div>

Title tidy is the quickest way to standardizes your media file names for use in Jellyfin, Plex, and Emby. Title tidy uses
intelligent parsing of folder structures and file names to automatically determine exactly how to name media. Whether you
need to rename a single episode, a whole season, or any number of shows, Title Tidy does the job in one command. A preview
is shown before renaming occurs, and Title Tidy will never overwrite content. 

The tool scans your current directory and displays an interactive preview showing exactly what will be renamed. The tool
reliably detects season and episode numbers across various formats (S01E01, 1x01, 101, etc.) and handles edge cases well.
Green items indicate pending changes. You can navigate through the list and apply changes when ready.

## Table of Contents

- [Basic Usage](#basic-usage)
- [Commands](#commands)
  - [Config](#config)
  - [Shows](#shows)
  - [Seasons](#seasons)
  - [Episodes](#episodes)
  - [Movies](#movies)
  - [Undo](#undo)
- [Logging](#logging)
- [Installation](#installation)
  - [Go Install (Recommended)](#go-install)
  - [Binary Release](#binary-release)
  - [Docker](#docker)
- [Built With](#built-with)
- [Contributing](#contributing)
- [License](#license)
- [Star History](#star-history)

## How to Use It

The tool provides five main commands: four for renaming different types of media, and one for undoing recent changes. Run it 
in the directory containing your media files, and you'll see a preview of all proposed changes. Nothing gets renamed until 
you confirm.

### Basic Usage

```bash
title-tidy [command]
```

**Available commands:**
* `shows` - Rename TV show directories with seasons and episodes
* `seasons` - Rename season folders and their episodes
* `episodes` - Rename episode files in current directory
* `movies` - Rename movie files and organize into folders
* `undo` - Revert recent rename operations
* `config` - Configure naming templates and logging settings

**Flags for rename commands:**
* Add the `-i` or `--instant` flag to apply changes immediately without the interactive preview.
* The `--no-nfo` flag will delete nfo files during the rename process.
* The `--no-img` flag will delete image files during the rename process.
* The `--no-sample` flag will delete files with "sample" in the name during the rename process.
* The `--link [DESTINATION]` flag will cause title-tidy to hard link files into the destination instead of renaming files in place. Use this if you are still seeding media files, but want to move them into your organized media.

## Commands

### Config

```bash
title-tidy config
```

Title Tidy allows you to completely customize how your media files are named using configurable templates, metadata enrichment from multiple providers, and logging settings. Your configuration is saved to `~/.title-tidy/config.json` and will be used for all future renames.

![config demo](https://vhs.charm.sh/vhs-2qcRVzJI8fANyCOWQXO7wi.gif)

This opens an interactive interface with multiple configuration sections:

#### Naming Templates

Customize how your media files and folders are named:

* **Show folders**: How TV show directories are named (default: `{title} ({year})`)
* **Season folders**: How season directories are named (default: `Season {season}`)
* **Episode files**: How individual episodes are named (default: `S{season}E{episode}`)
* **Movie folders**: How movie directories are named (default: `{title} ({year})`)

#### Available Template Variables

**Core Variables (always available):**
* `{title}` - Show or movie title
* `{year}` - Release year
* `{season}` - Season number with zero padding (e.g., "01")
* `{episode}` - Episode number with zero padding (e.g., "01")

**TMDB Metadata Variables (when TMDB is enabled):**
* `{title}` - Show or movie title
* `{episode_title}` - Episode title from TMDB (episodes only)
* `{air_date}` - Episode air date (episodes only)
* `{rating}` - TMDB rating score (e.g., "8.5")
* `{genres}` - Comma-separated genre list (e.g., "Drama, Crime")
* `{runtime}` - Runtime in minutes
* `{tagline}` - Movie tagline
* `{imdb_id}` - IMDB ID
* `{networks}` - TV Network that created the show (e.g., "HBO")

**OMDB Metadata Variables (when OMDB is enabled):**
* `{title}` - Show or movie title
* `{episode_title}` - Episode title straight from OMDB
* `{rating}` - IMDB user rating (episodes, shows, and movies)
* `{genres}` - Comma-separated genre list sourced from OMDB
* `{imdb_id}` - Canonical IMDB identifier for the title
* `{networks}` - TV network or distributor information for shows

**ffprobe Metadata Variables (when ffprobe is enabled):**
* `{video_codec}` - Video codec used in the media container file (episodes and movies only)
* `{video_resolution}` - Video resolution reported by ffprobe (episodes and movies only)
* `{audio_codec}` - Audio codec used in the media container file (episodes and movies only)

**Template Examples:**
* `S{season}E{episode}` → "S01E01"
* `{title} - S{season}E{episode} - {episode_title}` → "Breaking Bad - S01E01 - Pilot"
* `Season {season}` → "Season 01"
* `{title} ({year}) [{rating}]` → "The Matrix (1999) [8.7]"

#### Logging Settings

Configure operation history tracking for the undo feature:

* **Enable/Disable logging**: Toggle whether rename operations are tracked
* **Retention days**: How long to keep log files (default: 30 days)

When logging is enabled, all rename operations are saved to `~/.title-tidy/logs/` allowing you to undo recent changes. Old logs are automatically cleaned up based on your retention settings.

#### TMDB Integration

Enhance your media naming with rich metadata from The Movie Database:

* **Enable TMDB lookup**: Toggle metadata fetching from TMDB
* **API Key**: Your TMDB API key (get one free at [themoviedb.org](https://www.themoviedb.org))
  - Note: Use the API Key (v3 auth), not the Read Access Token
* **Language**: Content language for metadata (default: "en-US")
  - Examples: "fr-FR" for French, "es-ES" for Spanish, "ja-JP" for Japanese

When TMDB is enabled, Title Tidy will automatically fetch metadata for your media files, including proper titles, episode names, ratings, genres, and more. This data can be used in your naming templates to create information-rich filenames.

#### OMDB Integration

Unlock IMDB-powered metadata by connecting to the Open Movie Database:

* **Enable OMDB lookup**: Toggle metadata fetching from OMDB across movies, shows, seasons, and episodes
* **API Key**: Your OMDB API key (request one free at [omdbapi.com](https://www.omdbapi.com/apikey.aspx))

With OMDB enabled, Title Tidy enriches your media with IMDB ratings, genre lists, network information for TV shows, episode titles, and canonical IMDB identifiers. OMDB provides less metadata then TMDB, but is more privacy conscious. 

#### ffprobe Integration

The ffprobe integration only allows Enable/Disable in the configuration.

When ffprobe is enabled, Title Tidy will automatically scan your media files to extract the video resolution plus the video and audio codecs for use in file names.

### Shows

```bash
title-tidy shows
```

Use this when you have one or more complete TV shows with multiple seasons and episodes. It handles
the entire directory structure: show folders, season folders, and all episode files within. This
command can process multiple shows at once. Episode files named only after the episode
will retrieve the season number from the parent directory name. 

![shows demo](https://vhs.charm.sh/vhs-18YzRjjqaHeeA7T3oSkjAC.gif)

**Before → After examples:**
```
My.Cool.Show.2024.1080p.WEB-DL.x264/                → My Cool Show (2024)/
├── Season 1/                                       → ├── Season 01/
│   ├── Show.Name.S01E01.1080p.mkv                  → │   ├── S01E01.mkv
│   └── show.name.s01e02.mkv                        → │   └── S01E02.mkv
│   └── Show.Name.1x03.mkv                          → │   └── S01E03.mkv
│   └── 1.04.1080p.mkv                              → │   └── S01E04.mkv
├── s2/                                             → ├── Season 02/
│   ├── Episode 5.mkv                               → │   ├── S02E05.mkv
│   └── E06.mkv                                     → │   └── S02E06.mkv
├── Season_03 Extras/                               → ├── Season 03/
│   ├── Show.Name.S03E01.en.srt                     → │   ├── S03E01.en.srt
│   ├── Show.Name.S03E01.en-US.srt                  → │   ├── S03E01.en-US.srt
│   └── Show.Name.S03E02.srt                        → │   └── S03E02.srt
│   └── 10.12.mkv                                   → │   └── S10E12.mkv
Another-Show-2023-2024-2160p/                       → Another Show (2023-2024)/
├── Season-1/                                       → ├── Season 01/
│   ├── Show.Name.S01E01.mkv                        → │   ├── S01E01.mkv
│   └── Show.Name.1x02.mkv                          → │   └── S01E02.mkv
├── Season-2/                                       → ├── Season 02/
│   └── 2.03.mkv                                    → │   └── S02E03.mkv
Plain Show/                                         → Plain Show/
├── 5/                                              → ├── Season 05/
│   ├── Show.Name.S05E01.mkv                        → │   ├── S05E01.mkv
│   └── Episode 2.mkv                               → │   └── S05E02.mkv
Edge.Show/                                          → Edge Show/
├── Season 0/                                       → ├── Season 00/
│   └── S00E00.mkv                                  → │   └── S00E00.mkv
```

### Seasons

```bash
title-tidy seasons
```

Perfect when adding a new season to an existing show directory. Episode files named only after the episode
will retrieve the season number from the directory name. 

![seasons demo](https://vhs.charm.sh/vhs-1bRJ33A4Cjv08ADpQljJkr.gif)

**Before → After examples:**
```
Season_02_Test/                                     → Season 02/
├── Show.Name.S02E01.1080p.mkv                      → ├── S02E01.mkv
├── Show.Name.1x02.mkv                              → ├── S02E02.mkv
├── 2.03.mkv                                        → ├── S02E03.mkv
├── Episode 4.mkv                                   → ├── S02E04.mkv
├── E05.mkv                                         → ├── S02E05.mkv
└── Show.Name.S02E06.en.srt                         → └── S02E06.en.srt
```

### Episodes

```bash
title-tidy episodes
```

Sometimes you have a collection of episode files in a folder. No season directory, no show folder, just files.
This command renames each episode file based on the season and episode information found in the filename.

![episodes demo](https://vhs.charm.sh/vhs-1KolM7bO4Zho1BfR44p65R.gif)

**Before → After examples:**
```
Show.Name.S03E01.mkv                               → S03E01.mkv
show.name.s03e02.mkv                               → S03E02.mkv
3x03.mkv                                           → S03E03.mkv
3.04.mkv                                           → S03E04.mkv
Show.Name.S03E07.en-US.srt                         → S03E07.en-US.srt
```

### Movies

```bash
title-tidy movies
```

Movies receive special handling. Standalone movie files automatically get their own directories, while movies already in
folders have both the folder and file names cleaned up. Subtitles remain properly paired with their movies, maintaining
language codes.

![movies demo](https://vhs.charm.sh/vhs-3xo1AUhao1iUtafmkNHRdz.gif)

**Before → After examples:**
```
Another.Film.2023.720p.BluRay.mkv                  → Another Film (2023)/
                                                   → └── Another Film (2023).mkv
Plain_Movie-file.mp4                               → Plain Movie-file/
                                                   → └── Plain Movie-file.mp4
EdgeCase.Movie.2021.mkv                            → EdgeCase Movie (2021)/
EdgeCase.Movie.2021.en.srt                         → ├── EdgeCase Movie (2021).mkv
                                                   → └── EdgeCase Movie (2021).en.srt
Great.Movie.2024.1080p.x265/                       → Great Movie (2024)/
├── Great.Movie.2024.1080p.x265.mkv                → ├── Great Movie (2024).mkv
├── Great.Movie.2024.en.srt                        → ├── Great Movie (2024).en.srt
├── Great.Movie.2024.en-US.srt                     → ├── Great Movie (2024).en-US.srt
Some Film (2022)/                                  → Some Film (2022)/
├── Some.Film.2022.1080p.mkv                       → ├── Some Film (2022).mkv
```

### Undo

```bash
title-tidy undo
```

Accidentally renamed something? The undo command lets you revert recent rename operations. It displays a list of recent
rename sessions with details about what was changed, allowing you to select which session to undo.

![undo demo](https://vhs.charm.sh/vhs-52ZWSlz5ogGyGOpYqgKvzw.gif)

**Features:**
* View recent rename sessions with timestamps and file counts
* See detailed information about each session including command used and directory
* Select which session to revert
* Safe undo operation that only reverts successful renames
* Automatic cleanup of old log files based on retention settings

## Logging

Title Tidy automatically tracks all rename operations to enable the undo functionality. Each rename session is logged with:
* Timestamp of the operation
* Command and arguments used
* Working directory
* Complete list of all file operations (renames, links, deletions, directory creations)
* Success/failure status of each operation

Logs are stored in `~/.title-tidy/logs/` as JSON files and are automatically cleaned up based on your retention settings.

## Installation

### Go Install

The recommended way to install Title Tidy is using Go. This provides the best performance and easiest usage.

#### Installing Go

**For Windows:**
1. Visit [https://go.dev/dl/](https://go.dev/dl/)
2. Download the Windows installer (usually ends with `.msi`)
3. Run the installer and follow the prompts
4. Restart your computer after installation

**For macOS:**
1. Install [Homebrew](https://brew.sh/)
2. Run `brew install go` in your terminal

**For Linux:**
1. Visit [https://go.dev/dl/](https://go.dev/dl/)
2. Download the Linux tarball for your architecture
3. Extract and install.

#### Installing Title Tidy

Once Go is installed, you can install Title Tidy with a single command:

```bash
go install github.com/Digital-Shane/title-tidy@latest
```

### Binary Release

Download pre-built binaries from the [releases page](https://github.com/Digital-Shane/title-tidy/releases).

**Note:** On macOS, you'll need to bypass Gatekeeper since the binaries aren't signed:
```bash
# After downloading, remove quarantine attribute
xattr -d com.apple.quarantine title-tidy
```

### Docker

For containerized environments or if you prefer not to install Go, you can use Docker:

```bash
# Pull the latest image
docker pull digitalnal/title-tidy:latest

# Run a command (replace /path/to/your/media with your actual media directory)
docker run -v /path/to/your/media:/app/media -w /app/media digitalnal/title-tidy:latest title-tidy shows
```

#### Docker Compose

For repeated usage, you can use Docker Compose for easy interactive access:

```yaml
services:
  title-tidy:
    image: digitalnal/title-tidy:latest
    volumes:
      - ./media:/app/media
    working_dir: /app/media
    entrypoint: ["/bin/sh"]
```

Then run:
```bash
# Drop into an interactive shell in the container
docker-compose run --rm title-tidy

# Once inside the container, you can run title-tidy commands directly:
# title-tidy shows
# title-tidy seasons  
# title-tidy episodes
# exit
```

#### Building Locally

You can also build the Docker image locally:

```bash
docker build -t title-tidy .
docker run -v /path/to/your/media:/app/media -w /app/media title-tidy title-tidy shows
```

## Built With

This project is built using my [treeview](https://github.com/Digital-Shane/treeview) library, which provides
powerful tree structure visualization and manipulation capabilities in the terminal.

## Contributing

Contributions are welcome! If you have any suggestions or encounter a bug, please open an
[issue](https://github.com/Digital-Shane/title-tidy/issues) or submit a pull request.

When contributing:

1. Fork the repository and create a new feature branch
2. Make your changes in a well-structured commit history
3. Include tests (when applicable)
4. Submit a pull request with a clear description of your changes

## License

This project is licensed under the GNU Version 3 - see the [LICENSE](./LICENSE) file for details.

## Star History

[![Star History Chart](https://api.star-history.com/svg?repos=Digital-Shane/title-tidy&type=Date)](https://www.star-history.com/#Digital-Shane/title-tidy&Date)
