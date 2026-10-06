package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
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
		m.targetPane(leftWidth, bodyHeight),
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
	hints := "tab list · ↑/↓ select · I install · R run · s/x avd · r refresh · ? help · q quit"
	switch {
	case m.job != "":
		hints = styleGood.Render("● ") + "running " + m.job + " · esc cancel · q quit"
	case m.err != nil:
		hints = styleBad.Render("✗ " + m.err.Error())
	case m.status != "":
		hints = styleDim.Render(m.status)
	case !m.loaded:
		hints = styleDim.Render("loading…")
	}
	return fit(hints, m.width)
}

// targetPane is the left pane: one cursor-driven list over the three things a
// deployment needs — the devices to target, the AVDs that can be booted, and
// the APKs that can be installed. `tab` cycles them.
func (m Model) targetPane(outerWidth, outerHeight int) string {
	w, h := paneContentWidth(outerWidth), paneContentHeight(outerHeight)
	st := stylePane.Width(paneBoxWidth(outerWidth)).Height(paneBoxHeight(outerHeight))

	rows := []string{m.modeTabs(w)}
	switch m.mode {
	case listEmulators:
		rows = append(rows, m.listRows(w, h-1, len(m.emulators), m.avdCursor,
			"no AVDs found — n creates one", m.emulatorRow)...)
	case listAPKs:
		rows = append(rows, m.listRows(w, h-1, len(m.apks), m.apkCursor,
			"no APKs found — build the project, then r to rescan", m.apkRow)...)
	default:
		rows = append(rows, m.listRows(w, h-1, len(m.devices), m.devCursor,
			m.noDeviceReason(), m.deviceRow)...)
	}
	for len(rows) < h {
		rows = append(rows, "")
	}
	return st.Render(strings.Join(rows[:h], "\n"))
}

// modeTabs is the left pane's title: the three lists with their counts, the
// active one highlighted, and the list install/run deploy to marked with a dot.
// The mark follows the target even when the AVDS list is not the one on screen,
// because the APKS list only picks an APK and hides the target otherwise.
func (m Model) modeTabs(width int) string {
	parts := make([]string, 0, 3)
	for _, mode := range []listMode{listDevices, listEmulators, listAPKs} {
		label := fmt.Sprintf("%s (%d)", mode, m.count(mode))
		if mode == m.target {
			label = "● " + label
		}
		switch {
		case mode == m.mode:
			parts = append(parts, styleTitle.Render(label))
		case mode == m.target:
			parts = append(parts, styleGood.Render(label))
		default:
			parts = append(parts, styleDim.Render(label))
		}
	}
	return fit(strings.Join(parts, "  "), width)
}

func (m Model) count(mode listMode) int {
	switch mode {
	case listEmulators:
		return len(m.emulators)
	case listAPKs:
		return len(m.apks)
	}
	return len(m.devices)
}

// listRows renders the visible window of a list, or the empty-state note when
// it has nothing in it. The note is wrapped rather than truncated: it is the
// only thing explaining why the list is empty.
func (m Model) listRows(width, height, n, cursor int, empty string, row func(i, width int) string) []string {
	if n == 0 {
		return strings.Split(styleDim.Width(width).Render(empty), "\n")
	}
	start, end := visibleRange(n, cursor, height)
	rows := make([]string, 0, end-start)
	for i := start; i < end; i++ {
		rows = append(rows, row(i, width))
	}
	return rows
}

// emulatorRow is one line of the AVDS list.
func (m Model) emulatorRow(i, width int) string {
	e := m.emulators[i]
	marker := "  "
	if i == m.avdCursor {
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
	prefix := marker + padRight(ansi.Truncate(e.ID, idWidth, "…"), idWidth) + e.APILevel + "  "
	return stateRow(prefix, status, i == m.avdCursor, width)
}

// stateRow renders one list line: the leading columns, then the state that
// explains the row, coloured green / yellow / red by stateStyle. A selected row
// keeps its highlight — the state colour is inherited under the selection's
// background instead of replacing it.
func stateRow(prefix, state string, selected bool, width int) string {
	if !selected {
		return fit(prefix+stateStyle(state).Render(state), width)
	}
	return fit(styleSel.Render(prefix)+styleSel.Inherit(stateStyle(state)).Render(state), width)
}

// deviceRow is one line of the DEVICES list: the model, its serial (so two
// phones of the same model can be told apart) and the adb state. The label is
// what gets truncated — the state is the part that explains a refused install.
func (m Model) deviceRow(i, width int) string {
	d := m.devices[i]
	marker := "  "
	if i == m.devCursor {
		marker = "▸ "
	}
	state := d.State
	if d.Ready() {
		state = "ready"
	}
	label := d.Label()
	if label != d.Serial {
		label += "  " + d.Serial
	}
	labelWidth := width - ansi.StringWidth(state) - ansi.StringWidth(marker)
	if labelWidth < 8 {
		labelWidth = 8
	}
	prefix := marker + padRight(ansi.Truncate(label, labelWidth, "…"), labelWidth)
	return stateRow(prefix, state, i == m.devCursor, width)
}

// apkRow is one line of the APKS list: the path, then its size.
func (m Model) apkRow(i, width int) string {
	a := m.apks[i]
	marker := "  "
	if i == m.apkCursor {
		marker = "▸ "
	}
	size := humanSize(a.Size)
	pathWidth := width - ansi.StringWidth(size) - ansi.StringWidth(marker)
	if pathWidth < 8 {
		pathWidth = 8
	}
	line := marker + padRight(ansi.Truncate(a.Path, pathWidth, "…"), pathWidth) + size
	if i == m.apkCursor {
		return styleSel.Render(fit(line, width))
	}
	return fit(line, width)
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for m := n / unit; m >= unit; m /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGT"[exp])
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
		{"tab", "switch list: DEVICES / AVDS / APKS — the ● list is the target"},
		{"↑/↓, k/j", "move selection"},
		{"I", "install the selected APK on the target device or booted AVD"},
		{"R", "run: build, deploy & launch on the target device or booted AVD"},
		{"s / x", "start / stop the selected AVD"},
		{"r", "refresh devices, APKs, AVDs and environment"},
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
	b.WriteString("\n" + styleDim.Render("wraps the `android` CLI and adb · `android help` lists the full command surface"))
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
