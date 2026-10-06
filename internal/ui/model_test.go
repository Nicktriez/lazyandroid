package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nicktriez/lazyandroid/internal/android"
)

// deployModel is a sized model with one connected phone and one build on disk.
// The runner is /bin/true so a started job is real but harmless.
func deployModel() Model {
	m := New(android.Runner{Bin: "/bin/true"})
	m.width, m.height = 120, 40
	m.ready = true
	m.devices = []android.Device{
		{Serial: "34131JEGR08885", State: "device", Model: "Pixel_6a", Product: "bluejay"},
	}
	m.apks = []android.Artifact{
		{Path: "app/build/outputs/apk/debug/app-debug.apk", Size: 1 << 20},
	}
	return m
}

func TestInstallTargetsTheSelectedDeviceAndAPK(t *testing.T) {
	m := deployModel()
	mm, cmd := m.install()
	got := mm.(Model)
	if cmd == nil || got.stream == nil {
		t.Fatalf("install did not start a job (status %q)", got.status)
	}
	got.cancel()

	want := "$ android install --apks=app/build/outputs/apk/debug/app-debug.apk --device=34131JEGR08885"
	if len(got.output) == 0 || got.output[0] != want {
		t.Errorf("command = %q, want %q", got.output, want)
	}
	if got.job != "install → Pixel 6a" {
		t.Errorf("job = %q, want it to name the device", got.job)
	}
}

func TestRunTargetsTheSelectedDevice(t *testing.T) {
	m := deployModel()
	mm, cmd := m.run()
	got := mm.(Model)
	if cmd == nil || got.stream == nil {
		t.Fatalf("run did not start a job (status %q)", got.status)
	}
	got.cancel()

	want := "$ android run --apks=app/build/outputs/apk/debug/app-debug.apk --device=34131JEGR08885"
	if len(got.output) == 0 || got.output[0] != want {
		t.Errorf("command = %q, want %q", got.output, want)
	}

	// With no APK on disk the CLI is left to decide what to build.
	m = deployModel()
	m.apks = nil
	mm, cmd = m.run()
	got = mm.(Model)
	if cmd == nil {
		t.Fatal("run without an APK did not start")
	}
	got.cancel()
	if want := "$ android run --device=34131JEGR08885"; got.output[0] != want {
		t.Errorf("command = %q, want %q", got.output[0], want)
	}
}

// `android install` exits 0 even when it prints an error and installs nothing,
// so a missing APK has to be refused here rather than reported as a success.
func TestInstallRefusesWithoutAnAPK(t *testing.T) {
	m := deployModel()
	m.apks = nil

	mm, cmd := m.install()
	got := mm.(Model)
	if cmd != nil || got.stream != nil {
		t.Fatal("install started with no APK to install")
	}
	if !strings.Contains(got.status, "no APK") {
		t.Errorf("status = %q, want it to explain the missing APK", got.status)
	}
	if !strings.Contains(got.status, "APKS") {
		t.Errorf("status = %q, want it to point at the APKS list", got.status)
	}
}

// An unauthorised phone is listed, but targeting it just fails deep inside adb.
func TestInstallRefusesAnUnauthorisedDevice(t *testing.T) {
	m := deployModel()
	m.devices[0].State = "unauthorized"

	mm, cmd := m.install()
	got := mm.(Model)
	if cmd != nil || got.stream != nil {
		t.Fatal("install ran against an unauthorised device")
	}
	if !strings.Contains(got.status, "unauthorized") {
		t.Errorf("status = %q, want it to name the state", got.status)
	}
	if !strings.Contains(got.status, "USB debugging") {
		t.Errorf("status = %q, want it to say what to do", got.status)
	}
}

func TestRunRefusesWithoutADevice(t *testing.T) {
	m := deployModel()
	m.devices = nil
	m.adbErr = errors.New("adb not found (tried /nowhere/adb) — install it with `android sdk install platform-tools`")

	mm, cmd := m.run()
	got := mm.(Model)
	if cmd != nil || got.stream != nil {
		t.Fatal("run started with no device")
	}
	// A missing adb is a different problem from nothing being plugged in, so
	// the reason has to survive into the status.
	if !strings.Contains(got.status, "adb not found") {
		t.Errorf("status = %q, want the adb reason", got.status)
	}
}

// A job that has started must be tracked by the model the handler returns:
// without the stream there is no output, no job label and no esc to cancel.
func TestStartedJobsAreTrackedByTheReturnedModel(t *testing.T) {
	m := deployModel()
	m.emulators = []android.Emulator{{ID: "medium_phone", APILevel: "android-36"}}

	for _, tc := range []struct {
		key, job string
	}{
		{"s", "start medium_phone"},
		{"x", "stop medium_phone"},
		{"p", "sdk list --all"},
		{"d", "describe"},
		{"I", "install → Pixel 6a"},
		{"R", "run → Pixel 6a"},
	} {
		mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
		got := mm.(Model)
		if cmd == nil || got.stream == nil {
			t.Errorf("%q: the started job is not tracked (status %q)", tc.key, got.status)
			continue
		}
		if got.job != tc.job {
			t.Errorf("%q: job = %q, want %q", tc.key, got.job, tc.job)
		}
		got.cancel()
	}
}

func TestTabCyclesTheTargetLists(t *testing.T) {
	m := deployModel()
	for _, want := range []listMode{listDevices, listEmulators, listAPKs} {
		if m.mode != want {
			t.Fatalf("mode = %v, want %v", m.mode, want)
		}
		mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m = mm.(Model)
	}
	if m.mode != listDevices {
		t.Errorf("mode = %v, want it to wrap back to DEVICES", m.mode)
	}
}

// Arrows move the list the pane is showing and leave the others alone.
func TestMoveStepsTheActiveList(t *testing.T) {
	m := deployModel()
	m.apks = []android.Artifact{{Path: "a.apk"}, {Path: "b.apk"}}
	m.mode = listAPKs

	m.move(1)
	if m.apkCursor != 1 {
		t.Errorf("apkCursor = %d, want 1", m.apkCursor)
	}
	if m.devCursor != 0 || m.avdCursor != 0 {
		t.Errorf("other cursors moved: dev=%d avd=%d", m.devCursor, m.avdCursor)
	}

	// The cursor stays inside a list that shrank.
	m.apks = m.apks[:1]
	m.move(1)
	if m.apkCursor != 0 {
		t.Errorf("apkCursor = %d, want it clamped to 0", m.apkCursor)
	}
	m.apks = nil
	m.move(1)
	if m.apkCursor != 0 {
		t.Errorf("apkCursor = %d on an empty list, want 0", m.apkCursor)
	}
}

// avdModel is deployModel with the phone still plugged in and medium_phone
// booted, which is the reported situation: adb has a phone, the AVDS list has a
// running virtual device. The target starts on the DEVICES list, as the zero
// value does, so only `tab` can move it there.
func avdModel() Model {
	m := deployModel()
	m.emulators = []android.Emulator{{
		ID: "medium_phone", Name: "Medium Phone", APILevel: "android-36", Status: "Online", Serial: "emulator-5554",
	}}
	return m
}

// The reported bug: with a phone plugged in, tabbing to AVDS and pressing R
// deployed to the phone, because the DEVICES cursor was the only target.
func TestRunFollowsTheAVDSelectedInTheAVDSList(t *testing.T) {
	m := avdModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mm.(Model)
	if m.mode != listEmulators {
		t.Fatalf("mode = %v, want AVDS", m.mode)
	}

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	got := mm.(Model)
	if cmd == nil || got.stream == nil {
		t.Fatalf("run did not start a job (status %q)", got.status)
	}
	got.cancel()

	want := "$ android run --apks=app/build/outputs/apk/debug/app-debug.apk --device=emulator-5554"
	if len(got.output) == 0 || got.output[0] != want {
		t.Errorf("command = %q, want %q", got.output, want)
	}
	if got.job != "run → medium_phone" {
		t.Errorf("job = %q, want it to name the AVD", got.job)
	}
}

// The APKS list only picks an APK, so it must not re-aim the deploy at the
// phone: `tab` to APKS and `I` is the documented install loop.
func TestDeployTargetSurvivesTabbingToAPKS(t *testing.T) {
	m := avdModel()
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab}) // DEVICES -> AVDS
	m = mm.(Model)
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // AVDS -> APKS
	m = mm.(Model)
	if m.mode != listAPKs || m.target != listEmulators {
		t.Fatalf("mode = %v, target = %v, want APKS with the AVD still targeted", m.mode, m.target)
	}

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I")})
	got := mm.(Model)
	if cmd == nil || got.stream == nil {
		t.Fatalf("install did not start a job (status %q)", got.status)
	}
	got.cancel()

	want := "$ android install --apks=app/build/outputs/apk/debug/app-debug.apk --device=emulator-5554"
	if len(got.output) == 0 || got.output[0] != want {
		t.Errorf("command = %q, want %q", got.output, want)
	}
}

// An AVD that is not booted has no serial to target, so the AVDS list — where
// the user is plainly aiming at it — says so instead of deploying to the phone.
func TestInstallRefusesAnAVDThatIsNotRunning(t *testing.T) {
	m := avdModel()
	m.emulators[0].Status, m.emulators[0].Serial = "Offline", ""
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = mm.(Model)

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I")})
	got := mm.(Model)
	if cmd != nil || got.stream != nil {
		t.Fatal("install ran against an AVD that is not booted")
	}
	if !strings.Contains(got.status, "medium_phone is not running") || !strings.Contains(got.status, "press s") {
		t.Errorf("status = %q, want it to name the AVD and how to start it", got.status)
	}
}

// With no booted AVD there is nothing on the AVDS list to target, so a phone
// that is plugged in still takes the install instead of being refused.
func TestInstallFallsBackToThePhoneWithoutABootedAVD(t *testing.T) {
	m := avdModel()
	m.emulators[0].Status, m.emulators[0].Serial = "Offline", ""
	mm, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab}) // DEVICES -> AVDS
	m = mm.(Model)
	mm, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab}) // AVDS -> APKS
	m = mm.(Model)

	mm, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("I")})
	got := mm.(Model)
	if cmd == nil || got.stream == nil {
		t.Fatalf("install did not start a job (status %q)", got.status)
	}
	got.cancel()

	if !strings.Contains(got.output[0], "--device=34131JEGR08885") {
		t.Errorf("command = %q, want the phone", got.output[0])
	}
}

// The pane tabs mark which list owns the deploy target, so the choice stays
// visible from the APKS list, which has no target of its own.
func TestPaneTabsMarkTheDeployTarget(t *testing.T) {
	m := avdModel()
	m.mode = listAPKs
	m.target = listEmulators

	tabs := m.modeTabs(80)
	if !strings.Contains(tabs, "● AVDS") {
		t.Errorf("tabs = %q, want the AVDS list marked as the target", tabs)
	}
	if strings.Contains(tabs, "● DEVICES") || strings.Contains(tabs, "● APKS") {
		t.Errorf("tabs = %q, want only one target marked", tabs)
	}
}

// The footer is what tells the user which keys exist, so it has to fall back to
// the hints once a refresh answers — a status that is only ever set at startup
// would hide them for the whole session.
func TestFooterShowsKeyHintsOnceLoaded(t *testing.T) {
	m := New(android.Runner{})
	m.width, m.height = 120, 40
	m.ready = true

	if footer := m.footerView(); !strings.Contains(footer, "loading") {
		t.Errorf("footer = %q, want a loading note before anything answers", footer)
	}

	mm, _ := m.Update(emulatorsMsg{emus: []android.Emulator{{ID: "medium_phone"}}})
	m = mm.(Model)
	footer := m.footerView()
	if strings.Contains(footer, "loading") {
		t.Errorf("footer = %q, still loading after a refresh", footer)
	}
	if !strings.Contains(footer, "I install") || !strings.Contains(footer, "R run") {
		t.Errorf("footer = %q, want the key hints", footer)
	}

	// A real status still wins over the hints.
	m.status = "no device connected"
	if got := m.footerView(); !strings.Contains(got, "no device connected") {
		t.Errorf("footer = %q, want the status", got)
	}
}
