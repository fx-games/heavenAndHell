#!/bin/zsh
# Render every tinted tile (D1 monster variants and used D1 objects sharing their fresh icon) with its icon name
# and tint cause into a PNG table: ./debug-tints.sh [out.png]
cd "${0:A:h}" && go run -tags development . debug_tints "${1:-debug-tints.png}"
