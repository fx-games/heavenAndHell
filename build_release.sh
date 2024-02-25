#!/bin/sh

rm -rf ./builds
mkdir builds
mkdir builds/mac
mkdir builds/win
mkdir builds/win7
mkdir builds/linux

# NOTE: This is written from the perspective of a MacOS X workstation

# What's needed for compilation?
# Build flags:
# -tags ebitensinglethread - Should always be included
# -tags embedded - When building standalone executables with embedded resources
# GOOS, GOARCH - Set this to the target OS and Architecture
# CC - Set this to the cross-compiler for the target OS
# CGO_ENABLED=1 - Enable CGO for cross-compilation

GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o ./builds/mac/LuxAeterna -tags ebitensinglethread .

# Windows Include Files are here: /usr/local/Cellar/mingw-w64/11.0.1/toolchain-x86_64/x86_64-w64-mingw32/include
# Could be injected like this: -ldflags="-Ipath/to/headerfile -Lpath/to/lib"
# But seems to work without this for now..
CC=/usr/local/Cellar/mingw-w64/11.0.1/toolchain-x86_64/bin/x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags '-s -w' -o ./builds/win/LuxAeterna.exe -tags ebitensinglethread .

CC=/usr/local/Cellar/mingw-w64/11.0.1/toolchain-x86_64/bin/x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go1.20.12 build -trimpath -ldflags '-s -w' -o ./builds/win7/LuxAeterna.exe -tags ebitensinglethread .


./build_linux.sh

# We should now have the following files:
# - builds/mac/LuxAeterna
# - builds/win/LuxAeterna.exe
# - builds/linux/LuxAeterna

# copy assets
cp -r files/assets builds/mac/assets
cp -r files/assets builds/linux/assets
cp -r files/assets builds/win/assets
cp -r files/assets builds/win7/assets

# copy rogue windows
cp exclude/RogueWindows/rw-windows-amd64.exe builds/win/RogueWindows.exe
cp exclude/RogueWindows/*.cmd builds/win

cp exclude/RogueWindows/rw-windows7-amd64.exe builds/win7/RogueWindows.exe
cp exclude/RogueWindows/*.cmd builds/win7

cp exclude/RogueWindows/rw-linux-amd64 builds/linux/RogueWindows
cp exclude/RogueWindows/Stats exclude/RogueWindows/Log exclude/RogueWindows/Inventory exclude/RogueWindows/Equipment builds/linux

cp exclude/RogueWindows/rw-darwin-amd64 builds/mac/RogueWindows
cp exclude/RogueWindows/Stats exclude/RogueWindows/Log exclude/RogueWindows/Inventory exclude/RogueWindows/Equipment builds/mac


# make zip files
cd builds || exit
mv mac LuxAeterna-MacOSX
mv win LuxAeterna-Windows
mv win7 LuxAeterna-Windows-7
mv linux LuxAeterna-Linux
zip -r la-macos-amd64.zip LuxAeterna-MacOSX
zip -r la-windows-amd64.zip LuxAeterna-Windows
zip -r la-windows7-amd64.zip LuxAeterna-Windows-7
zip -r la-linux-amd64.zip LuxAeterna-Linux
cd ..