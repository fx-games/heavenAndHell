#!/bin/zsh
# Browse D1's own level generators in the terminal, one char per map cell.
# 1-4: cathedral, catacombs, caves, hell; the same key or space rerolls; q quits. Compare with scripts/levels_alt.sh.
cd "${0:A:h}/.." && go run ./src/cmd/levelcmp -d1 "$@"
