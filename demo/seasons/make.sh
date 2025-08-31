#!/usr/bin/env bash
# Generate a single-season focused tree for `rename-media seasons` - The Mandalorian
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/data"
rm -rf "$OUT" && mkdir -p "$OUT/The_Mandalorian_Season_02"
SEASON_DIR="$OUT/The_Mandalorian_Season_02"
# Episode files inside (depth 1)
touch "$SEASON_DIR/The.Mandalorian.S02E01.1080p.mkv"
touch "$SEASON_DIR/The.Mandalorian.1x02.mkv"
touch "$SEASON_DIR/2.03.mkv"      # dotted -> S02E03
touch "$SEASON_DIR/Episode 4.mkv" # context fallback
touch "$SEASON_DIR/E05.mkv"       # context fallback
touch "$SEASON_DIR/The.Mandalorian.S02E06.en.srt"

echo "Demo dataset for 'seasons' created at $OUT"
