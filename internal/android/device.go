package android

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Device is an attached device as reported by `adb devices -l`: a phone on the
// end of a USB cable, or an emulator that is already running. The android CLI
// only enumerates AVDs, so physical devices have to come from adb.
type Device struct {
	Serial  string
	State   string // "device" when usable; also "unauthorized", "offline", ...
	Model   string
	Product string
	Device  string
}

// Ready reports whether install/run can target the device. Anything else —
// "unauthorized" above all — means the phone has not accepted the host key.
func (d Device) Ready() bool { return d.State == "device" }

// Label is what the UI shows for the device: the model adb reports, with its
// underscores turned back into spaces, falling back to the serial.
func (d Device) Label() string {
	for _, s := range []string{d.Model, d.Product, d.Device} {
		if s != "" {
			return strings.ReplaceAll(s, "_", " ")
		}
	}
	return d.Serial
}

// Emulator reports whether the device is an emulator rather than a phone.
func (d Device) Emulator() bool { return strings.HasPrefix(d.Serial, "emulator-") }

// ADB runs the adb binary that ships in the Android SDK's platform-tools.
type ADB struct {
	Bin string
}

// Devices lists the attached devices via `adb devices -l`. adb keeps the list
// on stdout and its daemon chatter on stderr, so a device that is present is
// reported even when the call itself failed.
func (a ADB) Devices(ctx context.Context) ([]Device, error) {
	out, err := exec.CommandContext(ctx, a.Bin, "devices", "-l").Output()
	devices := ParseDevices(string(out))
	if err != nil {
		return devices, fmt.Errorf("%s devices -l: %w", filepath.Base(a.Bin), err)
	}
	return devices, nil
}

// ParseDevices parses `adb devices -l`. Each row is a serial, a state, then
// key:value attributes; the header and adb's "* daemon started" notices are
// skipped. "no permissions" is the one state that spans two words.
func ParseDevices(out string) []Device {
	var devices []Device
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "List of devices") || strings.HasPrefix(line, "*") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		d := Device{Serial: fields[0], State: fields[1]}
		attrs := fields[2:]
		if d.State == "no" && len(attrs) > 0 && attrs[0] == "permissions" {
			d.State = "no permissions"
			attrs = attrs[1:]
		}
		for _, attr := range attrs {
			key, value, ok := strings.Cut(attr, ":")
			if !ok {
				continue
			}
			switch key {
			case "model":
				d.Model = value
			case "product":
				d.Product = value
			case "device":
				d.Device = value
			}
		}
		devices = append(devices, d)
	}
	return devices
}

// FindADB locates the adb binary. adb lives in the SDK's platform-tools, which
// is rarely on PATH, so the SDK the android CLI reports is checked first; PATH
// is the last resort. sdkHint is `android info sdk`, empty when not yet known.
func FindADB(sdkHint string) (string, error) {
	var tried []string
	for _, candidate := range []string{
		os.Getenv("ANDROID_ADB"),
		sdkBin(sdkHint),
		sdkBin(os.Getenv("ANDROID_HOME")),
		sdkBin(os.Getenv("ANDROID_SDK_ROOT")),
		sdkBin(defaultSDKRoot()),
	} {
		if candidate == "" {
			continue
		}
		if isExecutable(candidate) {
			return candidate, nil
		}
		tried = append(tried, candidate)
	}
	if path, err := exec.LookPath("adb"); err == nil {
		return path, nil
	}
	return "", fmt.Errorf("adb not found (tried %s) — install it with `android sdk install platform-tools`",
		strings.Join(append(tried, "adb on PATH"), ", "))
}

func sdkBin(sdkRoot string) string {
	if sdkRoot == "" {
		return ""
	}
	return filepath.Join(sdkRoot, "platform-tools", "adb")
}

// defaultSDKRoot is Android Studio's usual Linux/macOS SDK location, used only
// until `android info` reports the real one.
func defaultSDKRoot() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, "Android", "Sdk")
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir() && info.Mode()&0o111 != 0
}
