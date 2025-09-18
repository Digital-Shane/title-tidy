#!/usr/bin/env bash
# Generate a flat directory (current working dir = season folder) for `rename-media episodes` - Better Call Saul
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/data"
rm -rf "$OUT" && mkdir -p "$OUT"

VIDEO_SRC="$OUT/.source-video.mp4"
"$DIR/../generate-test-video.sh" "$VIDEO_SRC"
# Put mixed episode naming forms directly inside the season dir
cp "$VIDEO_SRC" "$OUT/Better.Call.Saul.S03E01.mkv"
cp "$VIDEO_SRC" "$OUT/better.call.saul.s03e02.mkv"
cp "$VIDEO_SRC" "$OUT/Breaking.Bad.3x03.mkv"
cp "$VIDEO_SRC" "$OUT/Breaking.Bad.3.04.mkv"
touch "$OUT/Better.Call.Saul.S03E01.en-US.srt"

rm -f "$VIDEO_SRC"

echo "Demo dataset for 'episodes' created at $OUT"
echo "To test: cd '$OUT' && rename-media episodes"
