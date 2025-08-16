#!/usr/bin/env bash
# Generate a multi-show tree exercising varied naming patterns for `rename-media shows`.
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/data"
rm -rf "$OUT" && mkdir -p "$OUT"

# Show 1: Mixed separators, year + tags, multiple seasons, subtitles
SHOW1_RAW="My.Cool.Show.2024.1080p.WEB-DL.x264"
SHOW1_DIR="$OUT/$SHOW1_RAW"
mkdir -p "$SHOW1_DIR/Season 1" "$SHOW1_DIR/s2" "$SHOW1_DIR/Season_03 Extras"
# Season 1 episodes - direct SxxEyy + lowercase variant + alt 1x07 pattern + dotted
touch "$SHOW1_DIR/Season 1/Show.Name.S01E01.1080p.mkv"
touch "$SHOW1_DIR/Season 1/Show.Name.S01E01.1080p.nfo"
touch "$SHOW1_DIR/Season 1/Show.Name.S01E01.1080p.jpg"
touch "$SHOW1_DIR/Season 1/show.name.s01e02.mkv"
touch "$SHOW1_DIR/Season 1/show.name.s01e02.nfo"
touch "$SHOW1_DIR/Season 1/Show.Name.1x03.mkv"
touch "$SHOW1_DIR/Season 1/Show.Name.1x03.png"
touch "$SHOW1_DIR/Season 1/1.04.1080p.mkv"  # dotted pattern -> S01E04
touch "$SHOW1_DIR/Season 1/1.04.1080p.nfo"
# Season 2 episodes - context fallback (no season token in filename)
touch "$SHOW1_DIR/s2/Episode 5.mkv"    # S02E05 via parent
touch "$SHOW1_DIR/s2/Episode 5.nfo"
touch "$SHOW1_DIR/s2/E06.mkv"          # S02E06 via parent (E06)
touch "$SHOW1_DIR/s2/poster.jpg"
# Season 3 style alt name (Season_03) plus subtitles & dotted season 10 example (ignored here)
touch "$SHOW1_DIR/Season_03 Extras/10.12.mkv" # S10E12 (season 10) even though under Season 3 folder
touch "$SHOW1_DIR/Season_03 Extras/Show.Name.S03E01.en.srt"
touch "$SHOW1_DIR/Season_03 Extras/Show.Name.S03E01.en-US.srt"
touch "$SHOW1_DIR/Season_03 Extras/Show.Name.S03E02.srt"

# Show 2: Another naming style with year range & hyphens
SHOW2_RAW="Another-Show-2023-2024-2160p"
SHOW2_DIR="$OUT/$SHOW2_RAW"
mkdir -p "$SHOW2_DIR/Season-1" "$SHOW2_DIR/Season-2"
touch "$SHOW2_DIR/Season-1/Show.Name.S01E01.mkv"
touch "$SHOW2_DIR/Season-1/Show.Name.1x02.mkv"
touch "$SHOW2_DIR/Season-2/2.03.mkv" # dotted -> S02E03

# Show 3: Plain show, simple numbers only
SHOW3_RAW="Plain Show"
SHOW3_DIR="$OUT/$SHOW3_RAW"
mkdir -p "$SHOW3_DIR/5" # Season 5 via simple number
touch "$SHOW3_DIR/5/Show.Name.S05E01.mkv"
touch "$SHOW3_DIR/5/Episode 2.mkv" # context fallback S05E02

# Show 4: Single season with zero season/episode edge
SHOW4_RAW="Edge.Show"
SHOW4_DIR="$OUT/$SHOW4_RAW"
mkdir -p "$SHOW4_DIR/Season 0"
touch "$SHOW4_DIR/Season 0/S00E00.mkv"

echo "Demo dataset for 'shows' created at $OUT"
