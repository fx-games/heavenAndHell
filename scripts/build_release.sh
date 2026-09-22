#!/bin/sh
# Builds and zips a release for every platform from any machine, no cgo and no cross compilers needed (Ebiten loads
# the system's graphics and audio libraries at run time).
#   scripts/build_release.sh            the binary with a mods/ folder next to it
#   scripts/build_release.sh embedded   one self-contained binary, the mods built in (-tags embedded)
cd "$(dirname "$0")/.." || exit 1
set -e

TAGS=ebitensinglethread
OUT=builds
if [ "$1" = embedded ]; then
  TAGS="$TAGS,embedded"
  OUT=builds_embedded
fi

rm -rf "$OUT"
for target in darwin/amd64 darwin/arm64 linux/amd64 windows/amd64; do
  os=${target%/*} arch=${target#*/}
  dir="$OUT/OfHeavenAndHell-$os-$arch"
  exe=OfHeavenAndHell
  [ "$os" = windows ] && exe=OfHeavenAndHell.exe
  CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags '-s -w' -tags "$TAGS" -o "$dir/$exe" ./src
  if [ "$1" != embedded ]; then
    cp -R files/mods "$dir/mods" # everything the game reads at run time, next to the binary
  fi
  cp config.rec "$dir/config.rec"
  (cd "$OUT" && zip -qr "OfHeavenAndHell-$os-$arch.zip" "OfHeavenAndHell-$os-$arch")
done
ls -lh "$OUT"/*.zip
