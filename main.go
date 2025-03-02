package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/finnigja/gomat"
	"github.com/finnigja/gomat/symbols"

	"github.com/getlantern/systray"
	"github.com/shirou/gopsutil/v4/process"

	"golang.org/x/term"
)

const (
	deviceOnCommand  = "on"
	deviceOffCommand = "off"
	lightStatusOn    = "🟢"
	lightStatusOff   = "🔴"
)

// todo:
//  - retry sending Matter command a few times, in case first time fails..

var (
	appName         = "MeetingLight"
	inMeeting       = false
	lightOn         = false
	lightStatus     = lightStatusOff
	pollingInterval = 5 * time.Second
	appDir          string
	// matter bits
	ip              = "192.168.86.114"
	fabricId, _     = strconv.ParseUint("0x110", 0, 64)
	deviceId, _     = strconv.ParseUint("500", 0, 64)
	controllerId, _ = strconv.ParseUint("100", 0, 64)
	//systray bits
	mStatus *systray.MenuItem
)

func initApp() {

	dir, dirErr := os.UserConfigDir()
	var dirPath string
	if dirErr == nil {
		dirPath = filepath.Join(dir, appName)
	}
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		err := os.Mkdir(dirPath, 0755) // Use MkdirAll if creating nested directories
		if err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
		}
	}

	file, err := os.OpenFile(fmt.Sprintf("%s/%s", dirPath, "application.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	//defer file.Close()

	if term.IsTerminal(int(os.Stdout.Fd())) {
		log.SetOutput(io.MultiWriter(file, os.Stdout))
		log.Println("Terminal detected, logging to console and file.")
	} else {
		log.SetOutput(file)
		log.Println("Terminal not detected, logging to file.")
	}

	appDir = dirPath
}

func promptUser() (string, error) {
	cmd := exec.Command("osascript", "-e",
		`text returned of (display dialog "Enter light pairing code:" default answer "" with title "Setup a light..." buttons {"OK"} default button "OK")`)
	output, err := cmd.Output()
	return string(output), err
}

func createBasicFabric() *gomat.Fabric {
	cert_manager := gomat.NewFileCertManager(fabricId, appDir)
	err := cert_manager.Load()
	if err != nil {
		panic(err)
	}
	fabric := gomat.NewFabric(fabricId, cert_manager)
	return fabric
}

func connectDevice(fabric *gomat.Fabric) (gomat.SecureChannel, error) {
	// dumb retry 3 times if err when opening connection...
	secure_channel, err := gomat.StartSecureChannel(net.ParseIP(ip), 5540, 55555)
	if err != nil {
		time.Sleep(1 * time.Second)
		secure_channel, err = gomat.StartSecureChannel(net.ParseIP(ip), 5540, 55555)
		if err != nil {
			time.Sleep(1 * time.Second)
			secure_channel, err = gomat.StartSecureChannel(net.ParseIP(ip), 5540, 55555)
			if err != nil {
				return secure_channel, err
			}
		}
	}
	secure_channel, err = gomat.SigmaExchange(fabric, controllerId, deviceId, secure_channel)
	return secure_channel, err
}

// send a command to the Matter device
func sendDeviceCommand(command string) error {
	fabric := createBasicFabric()
	channel, err := connectDevice(fabric)
	if err != nil {
		return err
	}

	commandSymbol := symbols.COMMAND_ID_OnOff_Off
	if command == deviceOnCommand {
		commandSymbol = symbols.COMMAND_ID_OnOff_On
	}
	to_send := gomat.EncodeIMInvokeRequest(1, symbols.CLUSTER_ID_OnOff, uint32(commandSymbol), []byte{}, false, uint16(rand.Intn(0xffff)))

	err = channel.Send(to_send)
	if err != nil {
		channel.Close()
		return err
	}

	resp, err := channel.Receive()
	if err != nil {
		channel.Close()
		return err
	}
	_, err = resp.Tlv.GetIntRec([]int{1, 0, 1, 1, 0})
	if err != nil {
		channel.Close()
		return err
	}

	// make sure we close the channel when done, else UDP port-already-in-use errors
	channel.Close()
	return err
}

// check for indicators of active meetings
func isMeetingActive() bool {

	// checking for Zoom via process listing
	procs, err := process.Processes()
	if err != nil {
		log.Println("Checking for Zoom usage via process listing resulted in error: ", err)
	} else {
		for _, proc := range procs {
			name, err := proc.Name()
			if err != nil {
				continue
			}
			if strings.Contains(strings.ToLower(name), "cpthost") {
				return true
			}
		}
	}

	// checking for Meet via Google Chrome tabs listing
	script := `
		tell application "Google Chrome"
			set tabList to {}
			repeat with win in windows
				repeat with t in tabs of win
					set tabInfo to URL of t
					set end of tabList to tabInfo
				end repeat
			end repeat
			return tabList
		end tell
	`
	var out bytes.Buffer
	cmd := exec.Command("osascript", "-e", script)
	cmd.Stdout = &out
	err = cmd.Run()
	if err != nil {
		log.Println("Checking for Google Meet usage via Chrome tab list resulted in error: ", err)
	} else {
		resultStr := out.String()
		stringSlice := strings.Split(resultStr, ",")
		for _, v := range stringSlice {
			url := strings.TrimSpace(v)
			if strings.HasPrefix(url, "https://meet.google.com/") && !strings.HasPrefix(url, "https://meet.google.com/landing") {
				return true
			}
		}
	}

	// if we haven't found a meeting indicator & returned true already, then return false
	return false
}

// Toggle the nightlight
func toggleMatterLight(on bool) {
	command := deviceOffCommand
	if on {
		command = deviceOnCommand
	}

	err := sendDeviceCommand(command)
	if err != nil {
		log.Println(fmt.Sprintf("Error toggling Matter light: %s", err))
	}
}

// background task to monitor meetings
func monitorMeetings() {
	for {
		active := isMeetingActive()
		if active != inMeeting {
			if active {
				inMeeting = true
				lightOn = true
				updateLightStatus()
			} else {
				inMeeting = false
				lightOn = false
				updateLightStatus()
			}
		}
		time.Sleep(pollingInterval)
	}
}

func updateLightStatus() string {
	lightStatus = lightStatusOff
	if lightOn {
		lightStatus = lightStatusOn
		systray.SetIcon(IconOn)
	} else {
		systray.SetIcon(IconOff)
	}
	toggleMatterLight(lightOn)
	log.Println("Updating light status: ", lightStatus)
	if mStatus != nil {
		mStatus.SetTitle(fmt.Sprintf("Status: %s", lightStatus))
	}
	return lightStatus
}

func onReady() {
	systray.SetIcon(IconOff)
	//systray.SetTitle(appName)
	mStatus = systray.AddMenuItem(fmt.Sprintf("Status: %s", lightStatus), "")
	mStatus.Disable()
	mOverride := systray.AddMenuItemCheckbox("Light on regardless!", "", false)
	mSetup := systray.AddMenuItem("Setup...", "")
	mQuit := systray.AddMenuItem("Quit", "")

	for {
		select {
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		case <-mOverride.ClickedCh:
			if mOverride.Checked() {
				mOverride.Uncheck()
				lightOn = false
			} else {
				mOverride.Check()
				lightOn = true
			}
			updateLightStatus()
		case <-mSetup.ClickedCh:
			result, err := promptUser()
			if err != nil {
				log.Println(fmt.Sprintf("Setup was called & got error: %s", result))
			} else {
				log.Println(fmt.Sprintf("Setup was called & got result: %s", result))
			}
		}
	}

}

func onQuit() {
	os.Exit(0)
}

func main() {
	initApp()

	go monitorMeetings()

	systray.Run(onReady, onQuit)
}
