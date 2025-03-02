go build -o bin/meetinglight

# macOS packaging
TARGET="MeetingLight.app"
rm -rf $TARGET
mkdir $TARGET
mkdir $TARGET/Contents
cp package/Info.plist $TARGET/Contents/
mkdir $TARGET/Contents/MacOS
cp bin/meetinglight MeetingLight.app/Contents/MacOS/
mkdir $TARGET/Contents/Resources
cp package/icon.icns $TARGET/Contents/Resources
