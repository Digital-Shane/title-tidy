# Title Tidy

Title tidy is the quickest way to standardizes your media file names for use in Jellyfin, Plex, and Emby. Title tidy uses
intelligent parsing of folder structures and file names to automatically determine exactly how to name media. Whether you
need to rename a single episode, a whole season, or any number of shows, Title Tidy does the job in one command. A preview
is shown before renaming occurs, and Title Tidy will never overwrite content. 

The tool scans your current directory and displays an interactive preview showing exactly what will be renamed. The tool
reliably detects season and episode numbers across various formats (S01E01, 1x01, 101, etc.) and handles edge cases well.
Green items indicate pending changes. You can navigate through the list and apply changes when ready.

## How to Use It

The tool provides four main commands, each designed for different scenarios. Run it in the directory containing your
media files, and you'll see a preview of all proposed changes. Nothing gets renamed until you confirm.

### Basic Usage

```bash
title-tidy [command]
```

* Add the `-i` or `--instant` flag to apply changes immediately without the interactive preview.
* The `--no-nfo` flag will delete nfo files during the rename process.
* The `--no-img` flag will delete image files during the rename process.

## Commands

### Shows

```bash
title-tidy shows
```

Use this when you have one or more complete TV shows with multiple seasons and episodes. It handles
the entire directory structure: show folders, season folders, and all episode files within. This
command can process multiple shows at once. Episode files named only after the episode
will retrieve the season number from the parent directory name. 

![shows demo](https://vhs.charm.sh/vhs-5KIKITpGcbCmfDzfZrACo4.gif)

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

![seasons demo](https://vhs.charm.sh/vhs-2n8HxdATpEVDGOu9OSi8P4.gif)

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

![episodes demo](https://vhs.charm.sh/vhs-6bb7qr1mB6gpDDan3HAzO.gif)

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

![movies demo](https://vhs.charm.sh/vhs-5USdlv7mAxvQ2tybsXE7Ja.gif)

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

## Installation

### Prerequisites

Before installing Title Tidy, you need to have Go (Golang) installed on your computer.
Go is a programming language that Title Tidy is built with.

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

### Installing Title Tidy

Once Go is installed, you can install Title Tidy with a single command:

```bash
go install github.com/Digital-Shane/title-tidy@latest
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
