#!/usr/bin/env bash
# Generate a single-season focused tree for `rename-media seasons` - The Mandalorian
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/data"
rm -rf "$OUT" && mkdir -p "$OUT/The_Mandalorian_Season_02"
SEASON_DIR="$OUT/The_Mandalorian_Season_02"
VIDEO_SRC="$OUT/.source-video.mp4"
"$DIR/../generate-test-video.sh" "$VIDEO_SRC"
# Episode files inside (depth 1)
cp "$VIDEO_SRC" "$SEASON_DIR/The.Mandalorian.S02E01.1080p.mkv"
cp "$VIDEO_SRC" "$SEASON_DIR/The.Mandalorian.1x02.mkv"
cp "$VIDEO_SRC" "$SEASON_DIR/2.03.mkv"      # dotted -> S02E03
cp "$VIDEO_SRC" "$SEASON_DIR/Episode 4.mkv" # context fallback
cp "$VIDEO_SRC" "$SEASON_DIR/E05.mkv"       # context fallback
touch "$SEASON_DIR/The.Mandalorian.S02E03.en.srt"

rm -f "$VIDEO_SRC"

echo "Demo dataset for 'seasons' created at $OUT"
