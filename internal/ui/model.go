// Package ui implements the lazyandroid Bubble Tea interface.
package ui

import (
	"context"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/Nicktriez/lazyandroid/internal/android"
)

const (
	pollInterval = 5 * time.Second
	cmdTimeout   = 60 * time.Second
	maxOutput    = 2000
	apkScanLimit = 40
)

// listMode selects which target list the left pane shows. All three answer the
// same question — what can I deploy, and with what — so they share one
// cursor-driven list rather than three panes.
type listMode int

const (
	listDevices listMode = iota
	listEmulators
	listAPKs
)

func (l listMode) String() string {
	switch l {
	case listEmulators:
		return "AVDS"
	case listAPKs:
		return "APKS"
	}
	return "DEVICES"
}

// next cycles the left pane's list.
func (l listMode) next() listMode { return (l + 1) % 3 }

type (
	emulatorsMsg struct {
		emus []android.Emulator
		err  error
	}
	devicesMsg struct {
		devices []android.Device
		err     error
	}
	apksMsg struct {
		apks []android.Artifact
	}
	infoMsg struct {
		pairs []android.KeyValue
		err   error
	}
	tickMsg       time.Time
	outputLineMsg string
	outputDoneMsg struct{ err error }
)

// Model is the root Bubble Tea model.
type Model struct {
	runner android.Runner

	width, height int
	ready         bool
	help          bool

	mode listMode

	// target is the list install and run take their device from: the device
	// under the DEVICES cursor, or the booted AVD selected in the AVDS list.
	// `tab` sets it, so the list you opened is the one you are choosing from,
	// and the APKS list — which only picks an APK — leaves it alone. Without
	// it, tabbing to APKS to pick an APK would hand the deploy to the phone
	// that happens to be plugged in.
	target listMode

	devices   []android.Device
	devCursor int
	// adbErr is why the device list is empty, so a missing platform-tools
	// install is not mistaken for nothing being plugged in.
	adbErr error

	emulators []android.Emulator
	avdCursor int

	apks      []android.Artifact
	apkCursor int

	info []android.KeyValue

	output    []string
	outScroll int
	stream    <-chan string
	done      <-chan error
	cancel    context.CancelFunc
	job       string

	status string
	err    error
	// loaded is false until the first refresh answers. Until then the footer
	// shows a loading note rather than the key hints.
	loaded bool
}

// New builds the initial model.
func New(r android.Runner) Model {
	return Model{runner: r}
}

// Init loads the target lists and the environment, then starts the refresh loop.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshDevices(), m.refreshEmulators(), m.refreshAPKs(), m.refreshInfo(), tickCmd())
}

func tickCmd() tea.Cmd {
	return tea.Tick(pollInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func (m Model) refreshEmulators() tea.Cmd {
	runner := m.runner
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		emus, err := runner.Emulators(ctx)
		return emulatorsMsg{emus: emus, err: err}
	}
}

// refreshDevices lists the attached devices through adb. adb is not on PATH on
// a typical machine, so the SDK the android CLI reports is used to find it; the
// lookup is cheap and re-run each tick so a phone that is plugged in or
// authorised mid-session shows up.
func (m Model) refreshDevices() tea.Cmd {
	bin, err := android.FindADB(m.sdkPath())
	if err != nil {
		return func() tea.Msg { return devicesMsg{err: err} }
	}
	adb := android.ADB{Bin: bin}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		devices, err := adb.Devices(ctx)
		return devicesMsg{devices: devices, err: err}
	}
}

// sdkPath is the SDK the android CLI uses, as reported by `android info`.
func (m Model) sdkPath() string {
	for _, kv := range m.info {
		if kv.Key == "sdk" {
			return kv.Value
		}
	}
	return ""
}

// refreshAPKs rescans the current directory. It runs as a tea.Cmd because the
// walk touches the whole project tree.
func (m Model) refreshAPKs() tea.Cmd {
	return func() tea.Msg {
		return apksMsg{apks: android.FindAPKs(".", apkScanLimit)}
	}
}

func (m Model) refreshInfo() tea.Cmd {
	runner := m.runner
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), cmdTimeout)
		defer cancel()
		pairs, err := runner.Info(ctx)
		return infoMsg{pairs: pairs, err: err}
	}
}

// startJob streams `android <args...>` into the output pane. Only one job runs
// at a time; the returned command is nil when the job could not start.
func (m *Model) startJob(label string, args ...string) tea.Cmd {
	if m.stream != nil {
		m.status = "a job is already running — press esc to cancel"
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	st, err := m.runner.Stream(ctx, args...)
	if err != nil {
		cancel()
		m.pushLine("✗ " + err.Error())
		return nil
	}
	m.cancel = cancel
	m.stream, m.done = st.Lines, st.Done
	m.job = label
	m.outScroll = 0
	m.status = ""
	m.pushLine("$ android " + strings.Join(args, " "))
	return waitForLine(m.stream, m.done)
}

func (m *Model) pushLine(line string) {
	m.output = append(m.output, android.SanitizeText(line))
	if len(m.output) > maxOutput {
		m.output = m.output[len(m.output)-maxOutput:]
	}
}

func waitForLine(lines <-chan string, done <-chan error) tea.Cmd {
	return func() tea.Msg {
		line, ok := <-lines
		if !ok {
			return outputDoneMsg{err: <-done}
		}
		return outputLineMsg(line)
	}
}

// clampCursor keeps a cursor inside a list that may have shrunk or emptied.
func clampCursor(cursor, n int) int {
	if cursor >= n {
		return max(0, n-1)
	}
	if cursor < 0 {
		return 0
	}
	return cursor
}

// move steps the cursor of whichever list the left pane is showing.
func (m *Model) move(delta int) {
	switch m.mode {
	case listEmulators:
		m.avdCursor = clampCursor(m.avdCursor+delta, len(m.emulators))
	case listAPKs:
		m.apkCursor = clampCursor(m.apkCursor+delta, len(m.apks))
	default:
		m.devCursor = clampCursor(m.devCursor+delta, len(m.devices))
	}
}

// selectedAVD is the AVD under the cursor in the AVDS list.
func (m Model) selectedAVD() (android.Emulator, bool) {
	if m.avdCursor < 0 || m.avdCursor >= len(m.emulators) {
		return android.Emulator{}, false
	}
	return m.emulators[m.avdCursor], true
}

// selectedDevice is the device under the cursor in the DEVICES list.
func (m Model) selectedDevice() (android.Device, bool) {
	if m.devCursor < 0 || m.devCursor >= len(m.devices) {
		return android.Device{}, false
	}
	return m.devices[m.devCursor], true
}

// selectedAPK is the APK under the cursor in the APKS list.
func (m Model) selectedAPK() (android.Artifact, bool) {
	if m.apkCursor < 0 || m.apkCursor >= len(m.apks) {
		return android.Artifact{}, false
	}
	return m.apks[m.apkCursor], true
}

// noDeviceReason explains an empty DEVICES list: no adb at all is a different
// problem from nothing being plugged in.
func (m Model) noDeviceReason() string {
	if m.adbErr != nil {
		return m.adbErr.Error()
	}
	return "no device — plug a phone in over USB, enable USB debugging, then accept the host key prompt"
}

// notReady explains a device that is listed but cannot be targeted yet; an
// unauthorised phone is the usual reason an install "does nothing".
func notReady(d android.Device) string {
	return d.Label() + " is " + d.State + " — unlock the phone and accept the USB debugging prompt"
}

// deployTarget resolves the target of install and run: the serial passed to
// --device and the name the output pane labels the job with. The AVD selected
// in the AVDS list wins while it is booted — it carries the serial adb attaches
// it as, and is named by its ID, which is what the AVDS list shows. An AVD that
// is not booted has no serial, and is refused only while the AVDS list is the
// one on screen; from the APKS list the DEVICES cursor stands instead, so a
// phone that is plugged in still takes an install. The third result is the
// reason the target cannot be used, empty when it can.
func (m Model) deployTarget() (serial, label, reason string) {
	if m.mode == listEmulators || m.target == listEmulators {
		e, ok := m.selectedAVD()
		if ok && e.Running() {
			return e.Serial, e.ID, ""
		}
		if m.mode == listEmulators {
			if !ok {
				return "", "", "no AVD selected — n creates one"
			}
			return "", "", e.ID + " is not running — press s to start it, or tab to DEVICES to target a phone"
		}
	}
	dev, ok := m.selectedDevice()
	if !ok {
		return "", "", m.noDeviceReason()
	}
	if !dev.Ready() {
		return "", "", notReady(dev)
	}
	return dev.Serial, dev.Label(), ""
}

// install deploys the selected APK to the selected device. `android install`
// needs --apks and exits 0 even when it prints an error, so the missing pieces
// are refused here rather than reported as a success.
func (m Model) install() (tea.Model, tea.Cmd) {
	serial, label, reason := m.deployTarget()
	if reason != "" {
		m.status = reason
		return m, nil
	}
	apk, ok := m.selectedAPK()
	if !ok {
		m.status = "no APK found — build the project, then r to rescan (see the " + listAPKs.String() + " list)"
		return m, nil
	}
	args := android.WithDevice([]string{"install", "--apks=" + apk.Path}, serial)
	cmd := m.startJob("install → "+label, args...)
	return m, cmd
}

// run builds the current directory, deploys it and launches it on the selected
// device. The selected APK is passed when there is one; otherwise the CLI
// decides what to build.
func (m Model) run() (tea.Model, tea.Cmd) {
	serial, label, reason := m.deployTarget()
	if reason != "" {
		m.status = reason
		return m, nil
	}
	args := []string{"run"}
	if apk, ok := m.selectedAPK(); ok {
		args = append(args, "--apks="+apk.Path)
	}
	args = android.WithDevice(args, serial)
	cmd := m.startJob("run → "+label, args...)
	return m, cmd
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true

	case tickMsg:
		return m, tea.Batch(m.refreshDevices(), m.refreshEmulators(), tickCmd())

	case devicesMsg:
		m.loaded = true
		m.adbErr = msg.err
		if msg.err == nil {
			m.devices = msg.devices
			m.devCursor = clampCursor(m.devCursor, len(m.devices))
		}

	case apksMsg:
		m.loaded = true
		m.apks = msg.apks
		m.apkCursor = clampCursor(m.apkCursor, len(m.apks))

	case emulatorsMsg:
		m.loaded = true
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.emulators = msg.emus
		m.avdCursor = clampCursor(m.avdCursor, len(m.emulators))

	case infoMsg:
		m.loaded = true
		if msg.err == nil {
			m.info = msg.pairs
		}

	case outputLineMsg:
		m.pushLine(string(msg))
		if m.stream == nil {
			return m, nil
		}
		return m, waitForLine(m.stream, m.done)

	case outputDoneMsg:
		job := m.job
		m.stream, m.done, m.cancel, m.job = nil, nil, nil, ""
		if msg.err != nil {
			m.pushLine("✗ " + job + " failed: " + msg.err.Error())
		} else {
			m.pushLine("✓ " + job + " finished")
		}
		return m, tea.Batch(m.refreshDevices(), m.refreshAPKs())

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "ctrl+c":
		if m.cancel != nil {
			m.cancel()
		}
		return m, tea.Quit

	case "?":
		m.help = !m.help

	case "r":
		return m, tea.Batch(m.refreshDevices(), m.refreshEmulators(), m.refreshAPKs(), m.refreshInfo())

	case "i":
		return m, m.refreshInfo()

	case "tab":
		m.mode = m.mode.next()
		if m.mode == listAPKs {
			// A build may have finished since the last scan.
			return m, m.refreshAPKs()
		}
		// The list you open is the one you are choosing a deploy target from:
		// tab back to DEVICES to aim install/run at a phone instead of an AVD.
		m.target = m.mode

	case "up", "k":
		m.move(-1)

	case "down", "j":
		m.move(1)

	case "I":
		return m.install()

	case "R":
		return m.run()

	case "s":
		if e, ok := m.selectedAVD(); ok {
			cmd := m.startJob("start "+e.ID, "emulator", "start", e.ID)
			return m, cmd
		}
		m.status = "no AVD selected"

	case "x":
		if e, ok := m.selectedAVD(); ok {
			cmd := m.startJob("stop "+e.ID, "emulator", "stop", e.ID)
			return m, cmd
		}
		m.status = "no AVD selected"

	case "p":
		cmd := m.startJob("sdk list --all", "sdk", "list", "--all")
		return m, cmd

	case "d":
		cmd := m.startJob("describe", "describe")
		return m, cmd

	case "esc":
		if m.cancel != nil {
			m.cancel()
			m.status = "cancelling…"
		}

	case "pgup":
		m.outScroll += 10

	case "pgdown":
		m.outScroll = max(0, m.outScroll-10)

	case "end":
		m.outScroll = 0
	}
	return m, nil
}
