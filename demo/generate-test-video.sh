#!/usr/bin/env bash
# Generate a short test video for demo datasets.
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "usage: generate-test-video.sh <output-path>" >&2
  exit 1
fi

if ! command -v ffmpeg >/dev/null 2>&1; then
  echo "ffmpeg is required to generate demo videos" >&2
  exit 1
fi

OUTPUT=$1
mkdir -p "$(dirname "$OUTPUT")"

ffmpeg -hide_banner -loglevel error \
  -f lavfi -i color=c=black:s=1280x720:r=30:d=2 \
  -f lavfi -i anullsrc=channel_layout=stereo:sample_rate=48000 \
  -c:v libx264 -pix_fmt yuv420p -profile:v baseline -level 3.0 \
  -c:a aac -b:a 128k -shortest "$OUTPUT"
