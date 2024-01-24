#!/bin/bash

output_path=./output.iconset
image="$1"

mkdir "$output_path"

# the convert command comes from imagemagick
for size in 16 32 64 128 256; do
  half="$(($size / 2))"
  convert "$image" -resize x$size $output_path/icon_${size}x${size}.png
  convert "$image" -resize x$size $output_path/icon_${half}x${half}@2x.png
done
