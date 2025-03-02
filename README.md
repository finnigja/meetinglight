
app logs to hardcoded location
tail -f ~/Library/Application\ Support/MeetingLight/application.log


macOS packaging:
https://medium.com/@mattholt/packaging-a-go-application-for-macos-f7084b00f6b5



use sips to 1) convert SVG to PNG then 2) pad out PNG to make it sqare, w/out messing up scale

sips -s format png -o lightbulb-solid.png lightbulb-solid.svg
sips --padToHeightWidth 512 512 lightbulb-solid.png --out lightbulb-solid-square.png

