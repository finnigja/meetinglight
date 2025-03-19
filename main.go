package main

import (
	"bytes"
	"encoding/hex"
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
	"github.com/finnigja/gomat/discover"
	"github.com/finnigja/gomat/onboarding_payload"
	"github.com/finnigja/gomat/symbols"

	"github.com/getlantern/systray"
	"github.com/shirou/gopsutil/v4/process"

	"golang.org/x/term"
)

const (
	deviceOnCommand  = "on"
	deviceOffCommand = "off"
	lightStatusOn    = "ON"  // "🟢"
	lightStatusOff   = "OFF" // "🔴"
	lightStatusNone  = "NOT PAIRED"
)

// todo:
//  - retry sending Matter command a few times, in case first time fails..

var (
	appName         = "MeetingLight"
	inMeeting       = false
	lightOn         = false
	lightStatus     = lightStatusOff
	lightOverride   = false
	pollingInterval = 5 * time.Second
	appDir          string
	appConfig       = "device_ip"
	deviceSetup     = false
	// matter bits
	ip              net.IP
	fabricId, _     = strconv.ParseUint("0x110", 0, 64)
	userId, _       = strconv.ParseUint("100", 0, 64)
	deviceId, _     = strconv.ParseUint("500", 0, 64)
	controllerId, _ = strconv.ParseUint("100", 0, 64)
	//systray bits
	mStatus   *systray.MenuItem
	mOverride *systray.MenuItem
)

func initApp() {

	log.Println("initApp()!")

	dir, dirErr := os.UserConfigDir()
	if dirErr == nil {
		appDir = filepath.Join(dir, appName)
	} else {
		log.Fatal("could not open working directory")
	}
	if _, err := os.Stat(appDir); os.IsNotExist(err) {
		log.Println("appdir did not exist, creating")
		err := os.Mkdir(appDir, 0755) // Use MkdirAll if creating nested directories
		if err != nil {
			fmt.Printf("Error creating directory: %v\n", err)
		} else {
			createFabricCA()
		}
	}

	log.Println("opening log file for output...")
	file, err := os.OpenFile(fmt.Sprintf("%s/%s", appDir, "application.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
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

	configFile := filepath.Join(appDir, appConfig)
	data, err := os.ReadFile(configFile)
	if err != nil {
		log.Printf("warning: could not read config from file: %v\n", err)
	} else {
		log.Println("read config from file")
		ip = net.ParseIP(string(data))
		if ip == nil {
			log.Printf("could not read IP from config")
		} else {
			log.Printf("read IP from config, huzzah: %v\n", ip)
			deviceSetup = true
		}
	}

	log.Println("initApp() complete")
}

func promptUser() (string, error) {
	log.Println("promptUser()!")

	maxRetries := 10
	for i := 0; i < maxRetries; i++ {
		log.Println("getting devices")
		devices := discover.DiscoverAllComissionable("en0", true) // interface, ipv6_enabled
		if len(devices) == 1 {
			log.Println("got a device!!")
			devices[0].Dump()
			log.Println("IPs found: ", devices[0].Addrs)
			// grabbing index-1 should be IPv4.. might need to make this smarter
			ip = devices[0].Addrs[1]
			log.Println("setting up device with index-1 IP: ", ip)
			break
		}
		if i < maxRetries-1 {
			time.Sleep(pollingInterval)
		} else {
			log.Println("max retries reached, giving up")
			return "commissioning failed", fmt.Errorf("max retries reached & no device found")
		}
	}

	log.Println("getting passcode")
	// PIN=`./gomat decode-mc $1 | grep passcode | cut -d ' ' -f 2`
	cmd := exec.Command("osascript", "-e",
		`text returned of (display dialog "Found a device. Enter pairing code:" default answer "" with title "Setup a light..." buttons {"OK"} default button "OK")`)
	output, err := cmd.Output()
	if err != nil {
		return "commissioning failed", fmt.Errorf("no pin provided")
	}
	content := onboarding_payload.DecodeManualPairingCode(string(output))
	passcode := content.Passcode

	log.Println("commissioning device...")
	// ./gomat commission --ip $IP --pin $PIN --controller-id 100 --device-id 500
	fabric := createBasicFabric()
	err = gomat.Commission(fabric, ip, int(passcode), controllerId, deviceId)
	if err != nil {
		return "commissioning failed", err
	}

	cf := fabric.CompressedFabric()
	csf := hex.EncodeToString(cf)
	dids := fmt.Sprintf("%s-%016X", csf, deviceId)
	dids = strings.ToUpper(dids)
	fmt.Printf("device identifier: %s\n", dids)

	configFile := filepath.Join(appDir, appConfig)
	err = os.WriteFile(configFile, []byte(ip.String()), 0644)
	if err != nil {
		log.Printf("warning: could not write IP to file: %v\n", err)
	} else {
		log.Println("newly commissioned device IP written to file")
		deviceSetup = true
		updateLightStatus()
	}

	return "commissioning completed!", nil
}

// only call this if not already bootstrapped
func createFabricCA() {
	cert_manager := gomat.NewFileCertManager(fabricId, appDir)
	log.Println("bootstrapping with location: ", appDir)
	err := cert_manager.BootstrapCa()
	if err != nil {
		panic(err)
	}
	err = cert_manager.Load()
	if err != nil {
		panic(err)
	}
	fabric := gomat.NewFabric(fabricId, cert_manager)
	log.Println("creating new user with id: ", userId)
	err = fabric.CertificateManager.CreateUser(uint64(userId))
	if err != nil {
		panic(err)
	}
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
	log.Println("connecting to device with IP: ", ip)
	// dumb retry 3 times if err when opening connection...
	secure_channel, err := gomat.StartSecureChannel(ip, 5540, 55555)
	if err != nil {
		time.Sleep(1 * time.Second)
		secure_channel, err = gomat.StartSecureChannel(ip, 5540, 55555)
		if err != nil {
			time.Sleep(1 * time.Second)
			secure_channel, err = gomat.StartSecureChannel(ip, 5540, 55555)
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
				updateLightStatus()
			} else {
				inMeeting = false
				updateLightStatus()
			}
		}
		time.Sleep(pollingInterval)
	}
}

func updateLightStatus() string {
	log.Println("updateLightStatus()")
	if !deviceSetup {
		log.Println("setting status to none")
		lightStatus = lightStatusNone
		if mOverride != nil {
			mOverride.Disable()
		}
	} else {
		lightStatus = lightStatusOff
		if mOverride != nil {
			mOverride.Enable()
		}

		shouldBeOn := inMeeting || lightOverride
		if shouldBeOn != lightOn {
			lightOn = shouldBeOn
			if lightOn {
				lightStatus = lightStatusOn
				systray.SetIcon(IconOn)
			} else {
				lightStatus = lightStatusOff
				systray.SetIcon(IconOff)
			}
			toggleMatterLight(lightOn)
		}

		log.Println("Updating light status: ", lightStatus)
		if mStatus != nil {
			mStatus.SetTitle(fmt.Sprintf("Status: %s", lightStatus))
		}
	}
	return lightStatus
}

func onReady() {
	systray.SetIcon(IconOff)
	//systray.SetTitle(appName)
	mStatus = systray.AddMenuItem(fmt.Sprintf("Status: %s", lightStatus), "")
	mStatus.Disable()
	mOverride = systray.AddMenuItemCheckbox("Light on regardless!", "", false)
	if !deviceSetup {
		mOverride.Disable()
	}
	updateLightStatus()
	mSetup := systray.AddMenuItem("Pair a light...", "")
	mQuit := systray.AddMenuItem("Quit", "")

	for {
		select {
		case <-mQuit.ClickedCh:
			systray.Quit()
			return
		case <-mOverride.ClickedCh:
			if mOverride.Checked() {
				mOverride.Uncheck()
				lightOverride = false
			} else {
				mOverride.Check()
				lightOverride = true
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
	lightOverride = false
	updateLightStatus()
	log.Println("Shutting down gracefully...")
	return
}

func main() {
	initApp()

	if deviceSetup {
		log.Println("device is already configured, ready to go...")
	} else {
		log.Println("device not yet configured, need to use setup flow...")
	}

	go monitorMeetings()
	systray.Run(onReady, onQuit)
}
