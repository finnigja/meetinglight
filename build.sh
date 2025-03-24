rm ./bin/meetinglight*
rm ./MeetingLight.dmg

# build for the two target archs
CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o bin/meetinglight_arm64
CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/meetinglight_amd64

# use lipo to build a "fat" binary that works on both archs
lipo ./bin/meetinglight_arm64 ./bin/meetinglight_amd64 -create -output ./bin/meetinglight

# create macOS app
TARGET="MeetingLight.app"
rm -rf $TARGET
mkdir $TARGET
mkdir $TARGET/Contents
cp package/Info.plist $TARGET/Contents/
mkdir $TARGET/Contents/MacOS
cp bin/meetinglight MeetingLight.app/Contents/MacOS/
mkdir $TARGET/Contents/Resources
cp package/icon.icns $TARGET/Contents/Resources


# TODO codesign the .app
# eg. codesign --deep --force --verbose --sign "Developer ID Application: Your Name (TEAMID)" MeetingLight.app

# create dmg
hdiutil create -volname "MeetingLight Installer" -srcfolder MeetingLight.app MeetingLight.dmg


# TODO notarize & staple the dmg
# eg. xcrun notarytool submit MyApp.dmg --keychain-profile "AC_PASSWORD" --wait
# eg. xcrun stapler staple MyApp.dmg
