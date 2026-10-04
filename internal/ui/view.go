package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/Nicktriez/lazyandroid/internal/android"
)

// View renders the whole interface.
func (m Model) View() string {
	if !m.ready {
		return styleDim.Render("lazyandroid: waiting for terminal size…")
	}
	if m.help {
		return m.helpView()
	}

	header := m.headerView()
	footer := m.footerView()

	bodyHeight := m.height - lipgloss.Height(header) - lipgloss.Height(footer)
	if bodyHeight < 3 {
		bodyHeight = 3
	}

	leftWidth := m.width * 2 / 5
	if leftWidth < 32 {
		leftWidth = 32
	}
	if leftWidth > m.width-24 {
		leftWidth = m.width - 24
	}
	if leftWidth < 12 {
		leftWidth = 12
	}
	rightWidth := m.width - leftWidth

	body := lipgloss.JoinHorizontal(lipgloss.Top,
		m.emulatorPane(leftWidth, bodyHeight),
		m.detailPane(rightWidth, bodyHeight),
	)
	return header + "\n" + body + "\n" + footer
}

func (m Model) headerView() string {
	line := styleTitle.Render("lazyandroid")
	for _, kv := range m.info {
		if kv.Key == "version" {
			line += styleDim.Render("  android " + kv.Value)
			break
		}
	}
	return fit(line, m.width)
}

func (m Model) footerView() string {
	hints := "↑/↓ select · s start · x stop · r refresh · p sdk list · d describe · i env · ? help · q quit"
	switch {
	case m.job != "":
		hints = styleGood.Render("● ") + "running " + m.job + " · esc cancel · q quit"
	case m.err != nil:
		hints = styleBad.Render("✗ " + m.err.Error())
	case m.status != "":
		hints = styleDim.Render(m.status)
	}
	return fit(hints, m.width)
}

func (m Model) emulatorPane(outerWidth, outerHeight int) string {
	w, h := paneContentWidth(outerWidth), paneContentHeight(outerHeight)
	st := stylePane.Width(paneBoxWidth(outerWidth)).Height(paneBoxHeight(outerHeight))

	rows := []string{
		fit(styleTitle.Render("EMULATORS")+styleDim.Render(fmt.Sprintf("  (%d)", len(m.emulators))), w),
	}
	if len(m.emulators) == 0 {
		rows = append(rows, styleDim.Render("no AVDs found"))
	}
	start, end := visibleRange(len(m.emulators), m.cursor, h-1)
	for i := start; i < end; i++ {
		rows = append(rows, m.emulatorRow(i, m.emulators[i], w))
	}
	for len(rows) < h {
		rows = append(rows, "")
	}
	return st.Render(strings.Join(rows, "\n"))
}

func (m Model) emulatorRow(i int, e android.Emulator, width int) string {
	marker := "  "
	if i == m.cursor {
		marker = "▸ "
	}
	status := "offline"
	if e.Running() {
		status = "running"
	}
	right := e.APILevel + "  " + status
	idWidth := width - ansi.StringWidth(right) - ansi.StringWidth(marker)
	if idWidth < 8 {
		idWidth = 8
	}
	line := fit(marker+padRight(e.ID, idWidth)+right, width)
	if i == m.cursor {
		return styleSel.Render(line)
	}
	return line
}

func (m Model) detailPane(outerWidth, outerHeight int) string {
	w, h := paneContentWidth(outerWidth), paneContentHeight(outerHeight)
	st := stylePane.Width(paneBoxWidth(outerWidth)).Height(paneBoxHeight(outerHeight))

	infoHeight := len(m.info) + 1
	if infoHeight > h-3 {
		infoHeight = h - 3
	}
	if infoHeight < 1 {
		infoHeight = 1
	}
	return st.Render(m.infoView(w, infoHeight) + "\n" + m.outputView(w, h-infoHeight))
}

// infoView renders exactly height lines: the title, the environment pairs, and
// padding. It never wraps, so the caller can budget its lines exactly.
func (m Model) infoView(width, height int) string {
	rows := []string{fit(styleTitle.Render("ENVIRONMENT"), width)}
	for _, kv := range m.info {
		if len(rows) >= height {
			break
		}
		rows = append(rows, fit(styleDim.Render(kv.Key+": ")+kv.Value, width))
	}
	if len(m.info) == 0 && len(rows) < height {
		rows = append(rows, styleDim.Render("(none)"))
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return strings.Join(rows[:height], "\n")
}

// outputView renders exactly height lines: the title plus a window over the
// tail of the output, scrolled up by outScroll lines.
func (m Model) outputView(width, height int) string {
	label := "OUTPUT"
	if m.job != "" {
		label += " · " + m.job
	}
	rows := []string{fit(styleTitle.Render(label), width)}

	avail := height - 1
	if avail < 1 {
		avail = 1
	}
	end := min(len(m.output), max(0, len(m.output)-m.outScroll))
	start := max(0, end-avail)
	for _, line := range m.output[start:end] {
		rows = append(rows, fit(line, width))
	}
	for len(rows) < height {
		rows = append(rows, "")
	}
	return strings.Join(rows[:height], "\n")
}

func (m Model) helpView() string {
	bindings := [][2]string{
		{"↑/↓, k/j", "move selection"},
		{"s", "start selected AVD"},
		{"x", "stop selected AVD"},
		{"r", "refresh AVDs and environment"},
		{"i", "refresh environment"},
		{"p", "android sdk list --all"},
		{"d", "android describe"},
		{"esc", "cancel running job"},
		{"pgup/pgdn, end", "scroll output"},
		{"?", "toggle this help"},
		{"q, ctrl+c", "quit"},
	}
	var b strings.Builder
	b.WriteString(styleTitle.Render("lazyandroid — keys") + "\n\n")
	for _, kv := range bindings {
		b.WriteString("  " + padRight(styleDim.Render(kv[0]), 18) + kv[1] + "\n")
	}
	b.WriteString("\n" + styleDim.Render("wraps the `android` CLI · `android help` lists the full command surface"))
	return stylePane.Padding(1, 2).Render(b.String())
}

// visibleRange returns the [start,end) slice of n items to show in a list of
// at most height rows, keeping the cursor visible.
func visibleRange(n, cursor, height int) (int, int) {
	if n == 0 || height < 1 {
		return 0, 0
	}
	if n <= height {
		return 0, n
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	if start > n-height {
		start = n - height
	}
	return start, start + height
}

// paneBoxWidth is the Style.Width that renders a bordered pane exactly
// outer columns wide: lipgloss widths include padding but exclude the border.
func paneBoxWidth(outer int) int {
	if w := outer - stylePane.GetHorizontalBorderSize(); w > 1 {
		return w
	}
	return 1
}

func paneBoxHeight(outer int) int {
	if h := outer - stylePane.GetVerticalBorderSize(); h > 1 {
		return h
	}
	return 1
}

// paneContentWidth / paneContentHeight are the wrap and line budgets inside the
// pane, i.e. the outer size minus border and padding.
func paneContentWidth(outer int) int {
	if w := outer - stylePane.GetHorizontalFrameSize(); w > 1 {
		return w
	}
	return 1
}

func paneContentHeight(outer int) int {
	if h := outer - stylePane.GetVerticalFrameSize(); h > 1 {
		return h
	}
	return 1
}

// fit truncates a (possibly styled) string to width columns.
func fit(s string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(s, width, "…")
}

func padRight(s string, width int) string {
	if n := width - ansi.StringWidth(s); n > 0 {
		return s + strings.Repeat(" ", n)
	}
	return s
}
