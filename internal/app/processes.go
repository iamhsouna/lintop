package app

import (
	"fmt"
	"sort"
	"strings"
	"syscall"

	"github.com/mattn/go-runewidth"

	ui "github.com/metaspartan/gotui/v5"
	"lintop/internal/i18n"
)

func getThemeColorName(themeColor ui.Color) string {
	switch themeColor {
	case ui.ColorBlack:
		return "black"
	case ui.ColorRed:
		return "red"
	case ui.ColorGreen:
		return "green"
	case ui.ColorYellow:
		return "yellow"
	case ui.ColorBlue:
		return "blue"
	case ui.ColorMagenta:
		return "magenta"
	case ui.ColorSkyBlue:
		return "skyblue"
	case ui.ColorGold:
		return "gold"
	case ui.ColorSilver:
		return "silver"
	case ui.ColorWhite:
		return "white"
	case ui.ColorLime:
		return "lime"
	case ui.ColorOrange:
		return "orange"
	case ui.ColorViolet:
		return "violet"
	case ui.ColorPink:
		return "pink"
	default:
		return "white"
	}
}

func sortProcesses(processes []ProcessMetrics) {
	sort.Slice(processes, func(i, j int) bool {
		var less bool
		var equal bool

		switch columns[selectedColumn] {
		case "PID":
			less = processes[i].PID < processes[j].PID
			equal = processes[i].PID == processes[j].PID
		case "USER":
			u1, u2 := strings.ToLower(processes[i].User), strings.ToLower(processes[j].User)
			less = u1 < u2
			equal = u1 == u2
		case "VIRT":
			less = processes[i].VSZ > processes[j].VSZ // Descending default
			equal = processes[i].VSZ == processes[j].VSZ
		case "RES":
			less = processes[i].RSS > processes[j].RSS // Descending default
			equal = processes[i].RSS == processes[j].RSS
		case "CPU":
			less = processes[i].CPU > processes[j].CPU // Descending default
			equal = processes[i].CPU == processes[j].CPU
		case "GPU":
			less = processes[i].GPU > processes[j].GPU // Descending default
			equal = processes[i].GPU == processes[j].GPU
		case "MEM":
			less = processes[i].Memory > processes[j].Memory // Descending default
			equal = processes[i].Memory == processes[j].Memory
		case "TIME":
			iTime := parseTimeString(processes[i].Time)
			jTime := parseTimeString(processes[j].Time)
			less = iTime > jTime // Descending default
			equal = iTime == jTime
		case "CMD":
			c1, c2 := strings.ToLower(processes[i].Command), strings.ToLower(processes[j].Command)
			less = c1 < c2
			equal = c1 == c2
		default:
			less = processes[i].CPU > processes[j].CPU
			equal = processes[i].CPU == processes[j].CPU
		}

		if equal {
			// Secondary sort by PID (always ascending) to ensure stability
			return processes[i].PID < processes[j].PID
		}

		if sortReverse {
			return !less
		}
		return less
	})
}

func calculateMaxWidths(availableWidth int) map[string]int {
	maxWidths := map[string]int{
		"PID":  5,
		"USER": 8,
		"VIRT": 6,
		"RES":  6,
		"CPU":  6,
		"GPU":  6,
		"MEM":  5,
		"TIME": 11,
		"CMD":  15,
	}
	usedWidth := 0
	for col, width := range maxWidths {
		if col != "CMD" {
			usedWidth += width + 1
		}
	}

	cmdWidth := availableWidth - usedWidth
	if cmdWidth < 5 {
		cmdWidth = 5
	}
	maxWidths["CMD"] = cmdWidth
	return maxWidths
}

func buildHeader(maxWidths map[string]int, themeColorStr, selectedHeaderFg string) string {
	header := ""
	for i, col := range columns {
		width := maxWidths[col]

		// Determine arrow for selected column
		arrow := ""
		if i == selectedColumn {
			arrow = "↓"
			if sortReverse {
				arrow = "↑"
			}
		}

		// Build column text with arrow included in width
		colWithArrow := i18n.T("Process_"+col) + arrow

		w := runewidth.StringWidth(colWithArrow)
		padding := width - w
		if padding < 0 {
			padding = 0
		}

		colText := ""
		switch col {
		case "USER", "CMD": // Left-align
			colText = colWithArrow + strings.Repeat(" ", padding)
		default: // Right-align
			colText = strings.Repeat(" ", padding) + colWithArrow
		}

		header += fmt.Sprintf("[%s](fg:%s,bg:%s)", colText, selectedHeaderFg, themeColorStr)

		if i < len(columns)-1 {
			header += fmt.Sprintf("[%s](fg:%s,bg:%s)", "|", selectedHeaderFg, themeColorStr)
		}
	}
	return header
}

func buildProcessRows(processes []ProcessMetrics, maxWidths map[string]int) []string {
	items := make([]string, len(processes))
	for i, p := range processes {
		seconds := parseTimeString(p.Time)
		timeStr := formatTime(seconds)
		virtStr := formatMemorySize(p.VSZ)
		resStr := formatResMemorySize(p.RSS)
		username := runewidth.Truncate(p.User, maxWidths["USER"], "...")

		cmdName := p.Command // Already simplified by ps -c

		// Convert GPU from ms/s to percentage (ms/s / 10 = %)
		// 1000 ms/s = 100% GPU utilization
		gpuPercent := p.GPU / 10.0

		line := fmt.Sprintf("%*d %-*s %*s %*s %*.1f%% %*.1f%% %*.1f%% %*s %-s",
			maxWidths["PID"], p.PID,
			maxWidths["USER"], username,
			maxWidths["VIRT"], virtStr,
			maxWidths["RES"], resStr,
			maxWidths["CPU"]-1, p.CPU,
			maxWidths["GPU"]-1, gpuPercent,
			maxWidths["MEM"]-1, p.Memory,
			maxWidths["TIME"], timeStr,
			runewidth.Truncate(cmdName, maxWidths["CMD"], "..."),
		)

		if i == processList.SelectedRow-1 {
			items[i] = line
		} else if currentUser != "" && currentUser != "root" && p.User != currentUser {
			color := GetProcessTextColor(false)
			items[i] = fmt.Sprintf("[%s](fg:%s)", line, color)
		} else {
			color := GetProcessTextColor(true)
			items[i] = fmt.Sprintf("[%s](fg:%s)", line, color)
		}
	}
	return items
}

func updateProcessList() {
	processes := lastProcesses
	if searchText != "" {
		if filteredProcesses == nil {
			processes = []ProcessMetrics{}
		} else {
			processes = filteredProcesses
		}
	}

	if processes == nil {
		return
	}

	themeColorStr, selectedHeaderFg := resolveProcessThemeColor()

	termWidth, _ := GetCachedTerminalDimensions()
	availableWidth := termWidth - 2
	if availableWidth < 1 {
		availableWidth = 1
	}

	maxWidths := calculateMaxWidths(availableWidth)

	header := buildHeader(maxWidths, themeColorStr, selectedHeaderFg)
	sortProcesses(processes)
	rows := buildProcessRows(processes, maxWidths)

	items := make([]string, len(processes)+1)
	items[0] = header
	copy(items[1:], rows)

	processList.Title, processList.TitleStyle = getProcessListTitle()
	processList.Rows = items
}

func handleSearchInput(e ui.Event) {
	switch e.ID {
	case "<Escape>":
		searchMode = false
		searchText = ""
		filteredProcesses = nil
		updateProcessList()
	case "<Enter>":
		searchMode = false
		updateProcessList()
	case "<F9>":
		attemptKillProcess()
	case "<Backspace>":
		if len(searchText) > 0 {
			runes := []rune(searchText)
			searchText = string(runes[:len(runes)-1])
		}
		updateFilteredProcesses()
		updateProcessList()
	case "<Space>":
		searchText += " "
		updateFilteredProcesses()
		updateProcessList()
	default:
		// Only append printable characters (simple check)
		if len(e.ID) == 1 {
			searchText += e.ID
			updateFilteredProcesses()
			updateProcessList()
		}
	}
}

func refreshFilteredProcesses() {
	if searchText == "" {
		filteredProcesses = nil
		return
	}
	filteredProcesses = nil
	lowerText := strings.ToLower(searchText)
	for _, p := range lastProcesses {
		if strings.Contains(strings.ToLower(p.Command), lowerText) {
			filteredProcesses = append(filteredProcesses, p)
		}
	}
}

func updateFilteredProcesses() {
	refreshFilteredProcesses()
	if len(filteredProcesses) > 0 {
		processList.SelectedRow = 1
	} else {
		processList.SelectedRow = 0
	}
}

func updateKillModal() {
	termWidth, termHeight := GetCachedTerminalDimensions()
	modalWidth := 50
	modalHeight := 10 // Slightly taller for the buttons provided by widget

	x := (termWidth - modalWidth) / 2
	y := (termHeight - modalHeight) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	confirmModal.SetRect(x, y, x+modalWidth, y+modalHeight)

	// Theme colors
	var primaryColor ui.Color
	var bg ui.Color = CurrentBgColor
	// Ensure opacity
	if GetCurrentBgName() == "clear" {
		bg = ui.ColorBlack
	}

	if IsCatppuccinTheme(currentConfig.Theme) {
		primaryColor = processList.TitleStyle.Fg
	} else if IsLightMode && currentConfig.Theme == "white" {
		primaryColor = ui.ColorBlack
	} else if color, ok := colorMap[currentConfig.Theme]; ok {
		primaryColor = color
	} else {
		primaryColor = ui.ColorGreen
	}

	confirmModal.BackgroundColor = bg
	confirmModal.TextStyle = ui.NewStyle(ui.ColorWhite, bg)
	if IsLightMode {
		confirmModal.TextStyle = ui.NewStyle(ui.ColorBlack, bg)
	}
	confirmModal.BorderStyle = ui.NewStyle(primaryColor, bg)
	confirmModal.TitleStyle = ui.NewStyle(primaryColor, bg, ui.ModifierBold)
	// Style buttons
	for _, btn := range confirmModal.Buttons {
		btn.TextStyle = ui.NewStyle(primaryColor, bg)
		btn.ActiveStyle = ui.NewStyle(bg, primaryColor)
		btn.BorderStyle = ui.NewStyle(primaryColor, bg)
	}
}

func showKillModal(pid int) {
	killPending = true
	killPID = pid
	confirmModal.ActiveButtonIndex = 1

	if len(confirmModal.Buttons) >= 2 {
		confirmModal.Buttons[0].OnClick = func() {
			executeKill()
		}
		confirmModal.Buttons[1].OnClick = func() {
			hideKillModal()
			updateProcessList()
		}
	}

	confirmModal.Title = fmt.Sprintf(i18n.T("TUI_KillPIDTitle"), pid)
	updateKillModal()
}

func hideKillModal() {
	killPending = false
}

func handleKillPending(e ui.Event) {
	switch e.ID {
	case "y", "Y": // Quick confirm
		executeKill()
	case "n", "N", "<Escape>": // Quick cancel
		hideKillModal()
		updateProcessList()
	case "<Left>", "h":
		confirmModal.ActiveButtonIndex = 0
		updateKillModal()
	case "<Right>", "l":
		confirmModal.ActiveButtonIndex = 1
		updateKillModal()
	case "<Enter>", "<Space>":
		if confirmModal.ActiveButtonIndex >= 0 && confirmModal.ActiveButtonIndex < len(confirmModal.Buttons) {
			if confirmModal.Buttons[confirmModal.ActiveButtonIndex].OnClick != nil {
				confirmModal.Buttons[confirmModal.ActiveButtonIndex].OnClick()
			}
		}
	}
}

func executeKill() {
	if err := syscall.Kill(killPID, syscall.SIGTERM); err == nil {
		stderrLogger.Printf("Sent SIGTERM to PID %d\n", killPID)

		if procs, err := getProcessList(lastGPUMetrics.ActivePercent); err == nil {
			lastProcesses = procs
			if searchMode || searchText != "" {
				updateFilteredProcesses()
			}
		}
	} else {
		stderrLogger.Printf("Failed to kill PID %d: %v\n", killPID, err)
	}
	hideKillModal()
	updateProcessList()
}

func handleNavigation(e ui.Event) {
	if searchMode {
		return
	}

	switch e.ID {
	case "/":
		handleSearchToggle()
	case "<Escape>":
		handleSearchClear()
	case "<Up>", "k", "<MouseWheelUp>", "<Down>", "j", "<MouseWheelDown>", "g", "<Home>", "G", "<End>":
		handleVerticalNavigation(e)
	case "<Left>", "<Right>":
		handleColumnNavigation(e)
	case "<Enter>", "<Space>":
		handleSortToggle()
	case "<F9>":
		attemptKillProcess()
	}
}

func handleProcessListEvents(e ui.Event) {
	// Don't handle process list navigation when in Info or Fan layout (allow their own scrolling)
	if currentConfig.DefaultLayout == LayoutInfo || currentConfig.DefaultLayout == LayoutFan {
		return
	}
	if killPending {
		handleKillPending(e)
		return
	}
	if searchMode {
		handleSearchInput(e)
		return
	}
	handleNavigation(e)
}
