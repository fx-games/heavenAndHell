#!/bin/sh
# Build Heaven & Hell for the browser (Go js/wasm, H&H mod only, embedded) and upload it to
# https://ruzzoli.de/roguelikes/of-heaven-and-hell/. The page is web/: index.html (top bar, one/multi window),
# game.html (runs the wasm), fs.js (saves in IndexedDB, feeds to the page). `./deploy_web.sh build` skips the upload.
set -e
cd "$(dirname "$0")"
OUT=builds/web STAGE=builds/web-stage
rm -rf "$OUT" "$STAGE" && mkdir -p "$OUT" "$STAGE/files/mods"

# go:embed takes every mod under files/mods: stage a copy that holds only H&H
cp go.mod go.sum "$STAGE/" && cp -R src "$STAGE/" && cp files/*.go "$STAGE/files/"
cp -R files/mods/heavenandhell "$STAGE/files/mods/"
(cd "$STAGE" && GOOS=js GOARCH=wasm go build -tags embedded -trimpath -ldflags="-s -w" -o "../web/hh.wasm" ./src)
rm -rf "$STAGE"

cp web/index.html web/game.html web/fs.js "$(go env GOROOT)/lib/wasm/wasm_exec.js" "$OUT/"
# A first visit's config.rec: the repo's, with every feed the multi window mode shows switched on
sed -E -e 's/^(Feed(Inventory|Equipment|Character|VisibleItems|VisibleActors|Log)): .*/\1: true/' \
	-e 's/^Mod: .*/Mod: heavenandhell/' config.rec > "$OUT/defaults.rec"
ls -la "$OUT"
[ "$1" = build ] && exit 0

ssh ruzzoli.de 'sudo mkdir -p /var/www/ruzzoli.de/roguelikes/of-heaven-and-hell && sudo chown -R felix:www-data /var/www/ruzzoli.de/roguelikes/of-heaven-and-hell'
rsync -rtz --delete "$OUT/" ruzzoli.de:/var/www/ruzzoli.de/roguelikes/of-heaven-and-hell/
echo "https://ruzzoli.de/roguelikes/of-heaven-and-hell/"
