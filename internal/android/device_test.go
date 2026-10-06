package android

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// adbDevicesOutput is captured `adb devices -l` output: a phone, an emulator,
// an unauthorised device and an offline one.
const adbDevicesOutput = `List of devices attached
34131JEGR08885         device usb:8-5 product:bluejay model:Pixel_6a device:bluejay transport_id:1
emulator-5554          device product:sdk_gphone64_x86_64 model:sdk_gphone64_x86_64 device:emu64xa transport_id:2
0123456789ABCDEF       unauthorized usb:1-2 transport_id:3
ABCDEF0123456789       offline usb:1-3 transport_id:4

`

func TestParseDevices(t *testing.T) {
	devices := ParseDevices(adbDevicesOutput)
	if len(devices) != 4 {
		t.Fatalf("parsed %d devices, want 4: %+v", len(devices), devices)
	}

	phone := devices[0]
	if phone.Serial != "34131JEGR08885" || phone.State != "device" {
		t.Errorf("phone = %+v", phone)
	}
	if phone.Model != "Pixel_6a" || phone.Product != "bluejay" || phone.Device != "bluejay" {
		t.Errorf("phone attributes = %+v", phone)
	}
	if !phone.Ready() {
		t.Error("a device in state \"device\" is not targetable")
	}
	// adb encodes spaces as underscores; the UI shows the readable name.
	if got := phone.Label(); got != "Pixel 6a" {
		t.Errorf("Label() = %q, want %q", got, "Pixel 6a")
	}
	if phone.Emulator() {
		t.Error("a phone was reported as an emulator")
	}

	running := devices[1]
	if !running.Emulator() || !running.Ready() {
		t.Errorf("running emulator = %+v", running)
	}

	// An unauthorised device is listed but must never be targeted.
	unauth := devices[2]
	if unauth.Ready() {
		t.Error("an unauthorised device reported itself as targetable")
	}
	if got := unauth.Label(); got != unauth.Serial {
		t.Errorf("Label() = %q, want the serial %q when adb reports no model", got, unauth.Serial)
	}

	if devices[3].State != "offline" || devices[3].Ready() {
		t.Errorf("offline device = %+v", devices[3])
	}
}

// adb prints daemon notices around the device list; they are not devices.
func TestParseDevicesSkipsNoise(t *testing.T) {
	out := "* daemon not running; starting now at tcp:5037\n" +
		"List of devices attached\n" +
		"* daemon started successfully\n\n"
	if got := ParseDevices(out); len(got) != 0 {
		t.Errorf("parsed %d devices from daemon chatter: %+v", len(got), got)
	}
	// A real capture with a stale adb server keeps the device rows.
	out = "List of devices attached\n" + "SER1\tdevice usb:1-1\n" + "\n"
	if got := ParseDevices(out); len(got) != 1 || got[0].Serial != "SER1" {
		t.Errorf("tab-separated row not parsed: %+v", got)
	}
}

// "no permissions" is the one state that spans two words.
func TestParseDevicesNoPermissions(t *testing.T) {
	out := "List of devices attached\n" +
		"SER no permissions (user in plugdev group; are your udev rules wrong?); see [http://developer.android.com/tools/device.html]\n"
	got := ParseDevices(out)
	if len(got) != 1 {
		t.Fatalf("parsed %d devices, want 1: %+v", len(got), got)
	}
	if got[0].State != "no permissions" {
		t.Errorf("State = %q, want %q", got[0].State, "no permissions")
	}
}

func TestADBDevicesRunsTheBinary(t *testing.T) {
	bin := fakeADB(t, "#!/bin/sh\n"+
		"echo 'List of devices attached'\n"+
		"echo 'SERIAL device usb:1-1 model:Pixel_6a transport_id:1'\n")
	devices, err := ADB{Bin: bin}.Devices(context.Background())
	if err != nil {
		t.Fatalf("Devices: %v", err)
	}
	if len(devices) != 1 || devices[0].Model != "Pixel_6a" {
		t.Errorf("devices = %+v", devices)
	}
}

// A failing adb still prints whatever it managed to list, and the caller keeps
// both the list and the error.
func TestADBDevicesKeepsTheListOnFailure(t *testing.T) {
	bin := fakeADB(t, "#!/bin/sh\n"+
		"echo 'List of devices attached'\n"+
		"echo 'SERIAL device model:Pixel_6a'\n"+
		"exit 3\n")
	devices, err := ADB{Bin: bin}.Devices(context.Background())
	if err == nil {
		t.Fatal("a failing adb returned no error")
	}
	if len(devices) != 1 {
		t.Errorf("devices dropped on a failing call: %+v", devices)
	}
	if !strings.Contains(err.Error(), "adb devices -l") {
		t.Errorf("error = %q, want it to name the command", err)
	}
}

func TestFindADBUsesTheOverride(t *testing.T) {
	clearADBEnv(t)
	adb := filepath.Join(t.TempDir(), "my-adb")
	writeExecutable(t, adb)
	t.Setenv("ANDROID_ADB", adb)

	got, err := FindADB("")
	if err != nil {
		t.Fatalf("FindADB: %v", err)
	}
	if got != adb {
		t.Errorf("FindADB = %q, want the override %q", got, adb)
	}
}

// adb lives in the SDK's platform-tools and is rarely on PATH, so the SDK the
// android CLI reports has to be enough to find it.
func TestFindADBUsesTheSDKHint(t *testing.T) {
	clearADBEnv(t)
	sdk := t.TempDir()
	want := filepath.Join(sdk, "platform-tools", "adb")
	writeExecutable(t, want)

	got, err := FindADB(sdk)
	if err != nil {
		t.Fatalf("FindADB: %v", err)
	}
	if got != want {
		t.Errorf("FindADB = %q, want %q", got, want)
	}
}

func TestFindADBReportsMissing(t *testing.T) {
	clearADBEnv(t)
	t.Setenv("PATH", "")
	t.Setenv("HOME", t.TempDir()) // no ~/Android/Sdk fallback either

	_, err := FindADB("")
	if err == nil {
		t.Fatal("FindADB succeeded with no adb anywhere")
	}
	if !strings.Contains(err.Error(), "platform-tools") {
		t.Errorf("error = %q, want it to point at platform-tools", err)
	}
}

func fakeADB(t *testing.T, script string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "adb")
	if err := os.WriteFile(bin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake adb: %v", err)
	}
	return bin
}

func writeExecutable(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// clearADBEnv removes the ambient SDK hints so a test sees only what it sets.
func clearADBEnv(t *testing.T) {
	t.Helper()
	for _, key := range []string{"ANDROID_ADB", "ANDROID_HOME", "ANDROID_SDK_ROOT"} {
		t.Setenv(key, "")
	}
}
