#!/bin/sh

TARGET="MeetingLight.iconset"

mkdir $TARGET
sips -z 16 16 icon.png --out $TARGET/icon_16x16.png
sips -z 32 32 icon.png --out $TARGET/icon_32x32.png
sips -z 64 64 icon.png --out $TARGET/icon_64x64.png
sips -z 128 128 icon.png --out $TARGET/icon_128x128.png
sips -z 256 256 icon.png --out $TARGET/icon_256x256.png
sips -z 512 512 icon.png --out $TARGET/icon_512x512.png
sips -z 1024 1024 icon.png --out $TARGET/icon_1024x1024.png

iconutil -c icns MeetingLight.iconset

mv MeetingLight.icns icon.icns


# and create icon.go
# (go install github.com/cratonica/2goarray)
GOPATH=~/go
OUTPUT=icon.go
echo Generating $OUTPUT
echo "//+build linux darwin" > $OUTPUT
echo >> $OUTPUT
cat "lightbulb-regular.png" | $GOPATH/bin/2goarray Icon main >> $OUTPUT
mv $OUTPUT ../


rm -rf $TARGET
