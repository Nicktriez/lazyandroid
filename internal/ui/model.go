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
)

type (
	emulatorsMsg struct {
		emus []android.Emulator
		err  error
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

	emulators []android.Emulator
	cursor    int

	info []android.KeyValue

	output    []string
	outScroll int
	stream    <-chan string
	done      <-chan error
	cancel    context.CancelFunc
	job       string

	status string
	err    error
}

// New builds the initial model.
func New(r android.Runner) Model {
	return Model{runner: r, status: "loading…"}
}

// Init loads the AVD list and environment, then starts the refresh loop.
func (m Model) Init() tea.Cmd {
	return tea.Batch(m.refreshEmulators(), m.refreshInfo(), tickCmd())
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
	m.output = append(m.output, line)
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

func (m Model) selected() (android.Emulator, bool) {
	if m.cursor < 0 || m.cursor >= len(m.emulators) {
		return android.Emulator{}, false
	}
	return m.emulators[m.cursor], true
}

// Update handles messages.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.ready = true

	case tickMsg:
		return m, tea.Batch(m.refreshEmulators(), tickCmd())

	case emulatorsMsg:
		m.status = ""
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		m.err = nil
		m.emulators = msg.emus
		if m.cursor >= len(m.emulators) {
			m.cursor = max(0, len(m.emulators)-1)
		}

	case infoMsg:
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
		return m, m.refreshEmulators()

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
		return m, tea.Batch(m.refreshEmulators(), m.refreshInfo())

	case "i":
		return m, m.refreshInfo()

	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}

	case "down", "j":
		if m.cursor < len(m.emulators)-1 {
			m.cursor++
		}

	case "s":
		if e, ok := m.selected(); ok {
			cmd := m.startJob("start "+e.ID, "emulator", "start", e.ID)
			return m, cmd
		}
		m.status = "no AVD selected"

	case "x":
		if e, ok := m.selected(); ok {
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
