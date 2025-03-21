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


sips -s format png -o lightbulb-on.png lightbulb-on.svg
sips --padToHeightWidth 512 512 lightbulb-on.png --out lightbulb-on-square.png
sips -s format png -o lightbulb-off.png lightbulb-off.svg
sips --padToHeightWidth 512 512 lightbulb-off.png --out lightbulb-off-square.png
sips -s format png -o lightbulb-error.png lightbulb-error.svg
sips --padToHeightWidth 512 512 lightbulb-error.png --out lightbulb-error-square.png
sips -s format png -o lightbulb-unpaired.png lightbulb-unpaired.svg
sips --padToHeightWidth 512 512 lightbulb-unpaired.png --out lightbulb-unpaired-square.png


# and create icon.go
# (go install github.com/cratonica/2goarray)
GOPATH=~/go
OUTPUT=icon.go
echo Generating $OUTPUT
echo "//+build linux darwin" > $OUTPUT
echo >> $OUTPUT
cat "lightbulb-on-square.png" | $GOPATH/bin/2goarray IconOn main >> $OUTPUT
cat "lightbulb-off-square.png" | $GOPATH/bin/2goarray IconOff main >> $OUTPUT
cat "lightbulb-error-square.png" | $GOPATH/bin/2goarray IconError main >> $OUTPUT
cat "lightbulb-unpaired-square.png" | $GOPATH/bin/2goarray IconUnpaired main >> $OUTPUT
mv $OUTPUT ../


rm -rf $TARGET
