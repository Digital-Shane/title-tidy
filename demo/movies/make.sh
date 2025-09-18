#!/usr/bin/env bash
# Generate movie-oriented dataset for `title-tidy movies`.
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/data"
rm -rf "$OUT" && mkdir -p "$OUT"

VIDEO_SRC="$OUT/.source-video.mp4"
"$DIR/../generate-test-video.sh" "$VIDEO_SRC"

# Movie 1: The Shawshank Redemption - Directory with noisy name + video + subtitles languages
MOV1_RAW="The.Shawshank.Redemption.1994.1080p.x265"
mkdir -p "$OUT/$MOV1_RAW"
cp "$VIDEO_SRC" "$OUT/$MOV1_RAW/The.Shawshank.Redemption.1994.1080p.x265.mkv"
touch "$OUT/$MOV1_RAW/The.Shawshank.Redemption.1994.en.srt"
touch "$OUT/$MOV1_RAW/The.Shawshank.Redemption.1994.en-US.srt"

# Movie 2: Inception - Standalone file -> should create virtual directory
cp "$VIDEO_SRC" "$OUT/Inception.2010.720p.BluRay.mkv"

# Movie 3: Interstellar - Standalone file with mixed case ext
cp "$VIDEO_SRC" "$OUT/Interstellar_2014-file.mp4"

# Movie 4: The Dark Knight - Directory already clean
mkdir -p "$OUT/The Dark Knight (2008)"
cp "$VIDEO_SRC" "$OUT/The Dark Knight (2008)/The.Dark.Knight.2008.1080p.mkv"

# Movie 5: Pulp Fiction - Standalone with subtitle file pair
cp "$VIDEO_SRC" "$OUT/Pulp.Fiction.1994.mkv"
touch "$OUT/Pulp.Fiction.1994.en.srt"

rm -f "$VIDEO_SRC"

echo "Demo dataset for 'movies' created at $OUT"
