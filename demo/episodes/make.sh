#!/usr/bin/env bash
# Generate a flat directory (current working dir = season folder) for `rename-media episodes` - Better Call Saul
set -euo pipefail
DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUT="$DIR/data"
rm -rf "$OUT" && mkdir -p "$OUT"
# Put mixed episode naming forms directly inside the season dir
touch "$OUT/Better.Call.Saul.S03E01.mkv"
touch "$OUT/better.call.saul.s03e02.mkv"
touch "$OUT/Breaking.Bad.3x03.mkv"
touch "$OUT/Breaking.Bad.3.04.mkv"
touch "$OUT/Better.Call.Saul.S03E07.en-US.srt"

echo "Demo dataset for 'episodes' created at $OUT"
echo "To test: cd '$OUT' && rename-media episodes"
