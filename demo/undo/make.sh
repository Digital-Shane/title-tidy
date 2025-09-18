#!/usr/bin/env bash
# Generate movie-oriented dataset for `title-tidy movies`.
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/data"
rm -rf "$OUT" && mkdir -p "$OUT"

VIDEO_SRC="$OUT/.source-video.mp4"
"$DIR/../generate-test-video.sh" "$VIDEO_SRC"

# Movie 1: Directory with noisy name + video + subtitles languages
MOV1_RAW="Great.Movie.2024.1080p.x265"
mkdir -p "$OUT/$MOV1_RAW"
cp "$VIDEO_SRC" "$OUT/$MOV1_RAW/Great.Movie.2024.1080p.x265.mkv"
touch "$OUT/$MOV1_RAW/Great.Movie.2024.en.srt"
touch "$OUT/$MOV1_RAW/Great.Movie.2024.en-US.srt"

# Movie 2: Standalone file -> should create virtual directory
cp "$VIDEO_SRC" "$OUT/Another.Film.2023.720p.BluRay.mkv"

# Movie 3: Standalone file with mixed case ext and no year
cp "$VIDEO_SRC" "$OUT/Plain_Movie-file.mp4"

# Movie 4: Directory already clean
mkdir -p "$OUT/Some Film (2022)"
cp "$VIDEO_SRC" "$OUT/Some Film (2022)/Some.Film.2022.1080p.mkv"

# Movie 5: Standalone with subtitle file pair
cp "$VIDEO_SRC" "$OUT/EdgeCase.Movie.2021.mkv"
touch "$OUT/EdgeCase.Movie.2021.en.srt"

rm -f "$VIDEO_SRC"

echo "Demo dataset for 'movies' created at $OUT"
