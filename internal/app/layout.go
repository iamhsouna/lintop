package app

import (
	"fmt"

	ui "github.com/metaspartan/gotui/v5"
	w "github.com/metaspartan/gotui/v5/widgets"
	"lintop/internal/i18n"
)

const (
	LayoutDefault         = "default"
	LayoutAlternative     = "alternative"
	LayoutAlternativeFull = "alternative_full"
	LayoutVertical        = "vertical"
	LayoutCompact         = "compact"
	LayoutDashboard       = "dashboard"
	LayoutGaugesOnly      = "gauges_only"
	LayoutGPUFocus        = "gpu_focus"
	LayoutCPUFocus        = "cpu_focus"
	LayoutSmall           = "small"
	LayoutNetworkIO       = "network_io"
	LayoutInfo            = "info"
	LayoutTiny            = "tiny"         // Compact layout with abbreviated stats + mini process list
	LayoutMicro           = "micro"        // Ultra-compact gauges + sparklines, no process list
	LayoutNano            = "nano"         // Dense info panel + small gauges + mini process list
	LayoutPico            = "pico"         // Maximum density with 2x2 gauges + sparklines
	LayoutHistory         = "history"      // StepChart history for GPU, Power, and Memory
	LayoutHistoryFull     = "history_full" // StepChart history including CPU
	LayoutHistorySoC      = "history_soc"  // StepChart history: CPU, GPU, ANE, DRAM Bandwidth
	LayoutFan             = "fan"          // Fan control and temperature sensors
	LayoutGPUMemory       = "gpu_memory"   // GPU + Memory focused with memory bandwidth chart
	LayoutMultiGPU        = "multi_gpu"    // One pane per detected GPU + process list
)

var layoutOrder = []string{LayoutDefault, LayoutAlternative, LayoutAlternativeFull, LayoutVertical, LayoutCompact, LayoutDashboard, LayoutGaugesOnly, LayoutGPUFocus, LayoutCPUFocus, LayoutGPUMemory, LayoutMultiGPU, LayoutNetworkIO, LayoutSmall, LayoutTiny, LayoutMicro, LayoutNano, LayoutPico, LayoutHistory, LayoutHistoryFull, LayoutHistorySoC, LayoutFan}

// rowSpec pairs a layout weight with a widget for weighted row construction.
type rowSpec struct {
	weight float64
	widget any
}

// buildWeightedRows builds rows whose ratios are normalized to sum to 1.
// This lets layouts drop the ANE row without leaving empty space.
func buildWeightedRows(specs ...rowSpec) []any {
	total := 0.0
	for _, s := range specs {
		total += s.weight
	}
	if total <= 0 {
		total = 1
	}
	rows := make([]any, 0, len(specs))
	for _, s := range specs {
		rows = append(rows, ui.NewRow(s.weight/total, s.widget))
	}
	return rows
}

// equalCols returns equally-sized columns for the given widgets.
func equalCols(items ...any) []any {
	if len(items) == 0 {
		return nil
	}
	r := 1.0 / float64(len(items))
	cols := make([]any, 0, len(items))
	for _, it := range items {
		cols = append(cols, ui.NewCol(r, it))
	}
	return cols
}

// secondaryGauge returns the gauge that fills the secondary gauge slot: the
// ANE gauge on Apple Silicon, or the GPU temperature gauge on machines without
// an Apple Neural Engine but with a GPU.
func secondaryGauge() (*w.Gauge, bool) {
	if hasANE() {
		return aneGauge, true
	}
	if showGPUTempGauge() {
		return gpuTempGauge, true
	}
	return nil, false
}

// gaugeItems returns the standard gauge set, appending the secondary gauge
// (ANE usage or GPU temperature) when one is available.
func gaugeItems(base ...any) []any {
	items := append([]any{}, base...)
	if g, ok := secondaryGauge(); ok {
		items = append(items, g)
	}
	return items
}

func setupGrid() {
	totalLayouts = len(layoutOrder)
	for i, layout := range layoutOrder {
		if layout == currentConfig.DefaultLayout {
			currentLayoutNum = i
			break
		}
	}
	applyLayout(currentConfig.DefaultLayout)
}

func cycleLayout(step int) {
	currentIndex := 0
	for i, layout := range layoutOrder {
		if layout == currentConfig.DefaultLayout {
			currentIndex = i
			break
		}
	}
	n := len(layoutOrder)
	nextIndex := ((currentIndex+step)%n + n) % n
	currentConfig.DefaultLayout = layoutOrder[nextIndex]
	currentLayoutNum = nextIndex
	totalLayouts = n
	applyLayout(currentConfig.DefaultLayout)
	updateHelpText()
}

// switchToLayout jumps directly to the named layout. Pressing the shortcut
// again while already on it returns to the previously active layout.
func switchToLayout(layoutName string) {
	target := layoutName
	if currentConfig.DefaultLayout == layoutName {
		// Already on the target layout: toggle back to wherever we came from.
		// If that is unknown or would be a self-jump (e.g. the user reached
		// this layout by cycling with l/L, so no jump recorded it), fall back
		// to the default layout so the shortcut always leaves the layout
		// instead of deadlocking on itself.
		target = previousLayout
		if target == "" || target == layoutName {
			target = LayoutDefault
		}
	}
	previousLayout = currentConfig.DefaultLayout
	for i, layout := range layoutOrder {
		if layout == target {
			currentLayoutNum = i
			break
		}
	}
	currentConfig.DefaultLayout = target
	totalLayouts = len(layoutOrder)
	applyLayout(target)
	updateHelpText()
}

func applyLayout(layoutName string) {
	termWidth, termHeight := ui.TerminalDimensions()
	if mainBlock != nil {
		mainBlock.SetRect(0, 0, termWidth, termHeight)
		mainBlock.TitleBottomLeft = fmt.Sprintf(i18n.T("TUI_LayoutInfo"), currentLayoutNum+1, totalLayouts, currentColorName)
		if termWidth < 93 {
			mainBlock.TitleBottom = ""
		} else {
			mainBlock.TitleBottom = i18n.T("TUI_InfoLayoutColorExit")
		}
	}
	grid = ui.NewGrid()

	setLayoutGrid(layoutName)

	if termWidth > 2 && termHeight > 2 {
		grid.SetRect(1, 1, termWidth-1, termHeight-1)
	}
}

func setLayoutGrid(layoutName string) {
	switch layoutName {
	case LayoutAlternative:
		grid.Set(
			ui.NewRow(1.0/2,
				ui.NewCol(1.0/2, cpuCoreWidget),
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, gpuGauge),
					ui.NewCol(1.0, ui.NewRow(1.0, memoryGauge)),
				),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, modelText),
				ui.NewCol(1.0/3, NetworkInfo),
				ui.NewCol(1.0/4, PowerChart),
				ui.NewCol(1.0/4, sparklineGroup),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutAlternativeFull:
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, cpuCoreWidget),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, gpuGauge),
				ui.NewCol(1.0/2, memoryGauge),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/6, modelText),
				ui.NewCol(1.0/3, NetworkInfo),
				ui.NewCol(1.0/4, PowerChart),
				ui.NewCol(1.0/4, sparklineGroup),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutVertical:
		leftSpecs := []rowSpec{
			{1.0, cpuGauge},
			{1.0, gpuGauge},
		}
		if g, ok := secondaryGauge(); ok {
			leftSpecs = append(leftSpecs, rowSpec{1.0, g})
		}
		leftSpecs = append(leftSpecs,
			rowSpec{1.5, memoryGauge},
			rowSpec{1.5, NetworkInfo},
			rowSpec{2.0, modelText},
		)
		grid.Set(
			ui.NewRow(1.0,
				ui.NewCol(0.4, buildWeightedRows(leftSpecs...)...),
				ui.NewCol(0.6,
					ui.NewRow(3.0/4, processList),
					ui.NewRow(1.0/4,
						ui.NewCol(1.0/2, PowerChart),
						ui.NewCol(1.0/2, sparklineGroup),
					),
				),
			),
		)
	case LayoutCompact:
		grid.Set(
			ui.NewRow(2.0/8, equalCols(gaugeItems(cpuGauge, gpuGauge, memoryGauge)...)...),
			ui.NewRow(2.0/8,
				ui.NewCol(1.0/3, modelText),
				ui.NewCol(1.0/3, NetworkInfo),
				ui.NewCol(1.0/3, PowerChart),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutDashboard:
		grid.Set(
			ui.NewRow(1.0/4, equalCols(gaugeItems(cpuGauge, gpuGauge, memoryGauge)...)...),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, sparklineGroup),
				ui.NewCol(1.0/2, gpuSparklineGroup),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutGaugesOnly:
		var gpuRow any
		if g, ok := secondaryGauge(); ok {
			gpuRow = ui.NewRow(1.0/3,
				ui.NewCol(1.0/2, gpuGauge),
				ui.NewCol(1.0/2, g),
			)
		} else {
			gpuRow = ui.NewRow(1.0/3, ui.NewCol(1.0, gpuGauge))
		}
		grid.Set(
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/2, cpuGauge),
				ui.NewCol(1.0/2, memoryGauge),
			),
			gpuRow,
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/2, gpuSparklineGroup),
				ui.NewCol(1.0/2, sparklineGroup),
			),
		)
	case LayoutGPUFocus:
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, gpuGauge),
				ui.NewCol(1.0/2, gpuSparklineGroup),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/4, cpuGauge),
				ui.NewCol(1.0/4, memoryGauge),
				ui.NewCol(1.0/4, NetworkInfo),
				ui.NewCol(1.0/4, modelText),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutCPUFocus:
		grid.Set(
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/3, cpuGauge),
				ui.NewCol(2.0/3, cpuCoreWidget),
			),
			ui.NewRow(1.0/6,
				ui.NewCol(1.0/4, gpuGauge),
				ui.NewCol(1.0/4, memoryGauge),
				ui.NewCol(1.0/4, sparklineGroup),
				ui.NewCol(1.0/4, PowerChart),
			),
			ui.NewRow(3.0/6,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutNetworkIO:
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/3, gpuSparklineGroup),
				ui.NewCol(1.0/3, sparklineGroup),
				ui.NewCol(1.0/3, NetworkInfo),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, gpuGauge),
					ui.NewRow(1.0/2, memoryGauge),
				),
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, tbInfoParagraph),
					ui.NewRow(1.0/2, tbNetSparklineGroup),
				),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutSmall:
		specs := []rowSpec{
			{1.0, cpuGauge},
			{1.0, gpuGauge},
			{1.0, memoryGauge},
		}
		if g, ok := secondaryGauge(); ok {
			specs = append(specs, rowSpec{1.0, g})
		}
		grid.Set(
			ui.NewRow(1.0,
				ui.NewCol(1.0, buildWeightedRows(specs...)...),
			),
		)
	case LayoutTiny, LayoutMicro, LayoutNano, LayoutPico:
		setCompactLayoutGrid(layoutName)
	case LayoutInfo, LayoutFan:
		setInfoFanLayoutGrid(layoutName)
	case LayoutMultiGPU:
		setMultiGPULayoutGrid()
	case LayoutHistory, LayoutHistoryFull, LayoutGPUMemory:
		setHistoryLikeLayoutGrid(layoutName)
	case LayoutHistorySoC:
		setHistorySoCLayoutGrid()
	default: // LayoutDefault
		contentRow := ui.NewRow(1.0,
			ui.NewCol(1.0/2, PowerChart),
			ui.NewCol(1.0/2, sparklineGroup),
		)
		leftSpecs := []rowSpec{}
		if g, ok := secondaryGauge(); ok {
			leftSpecs = append(leftSpecs, rowSpec{1.0, g})
		}
		leftSpecs = append(leftSpecs, rowSpec{1.0, contentRow})
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, cpuGauge),
				ui.NewCol(1.0/2, gpuGauge),
			),
			ui.NewRow(2.0/4,
				ui.NewCol(1.0/2, buildWeightedRows(leftSpecs...)...),
				ui.NewCol(1.0/2,
					ui.NewRow(1.0/2, memoryGauge),
					ui.NewRow(1.0/2,
						ui.NewCol(1.0/3, modelText),
						ui.NewCol(2.0/3, NetworkInfo),
					),
				),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	}
}

func setCompactLayoutGrid(layoutName string) {
	switch layoutName {
	case LayoutTiny:
		// Compact vertical with all key metrics + mini process list
		row2 := []any{memoryGauge}
		if g, ok := secondaryGauge(); ok {
			row2 = append(row2, g)
		}
		grid.Set(
			ui.NewRow(1.0/4, equalCols(cpuGauge, gpuGauge)...),
			ui.NewRow(1.0/4, equalCols(row2...)...),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, PowerChart),
				ui.NewCol(1.0/2, NetworkInfo),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutMicro:
		// Ultra-compact gauges + sparklines, no process list
		row2 := []any{memoryGauge}
		if g, ok := secondaryGauge(); ok {
			row2 = append(row2, g)
		}
		grid.Set(
			ui.NewRow(1.0/4, equalCols(cpuGauge, gpuGauge)...),
			ui.NewRow(1.0/4, equalCols(row2...)...),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, sparklineGroup),
				ui.NewCol(1.0/2, gpuSparklineGroup),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, PowerChart),
				ui.NewCol(1.0/2, NetworkInfo),
			),
		)
	case LayoutNano:
		// Dense info panel + gauges + mini process list
		row3 := []any{PowerChart, NetworkInfo}
		if g, ok := secondaryGauge(); ok {
			row3 = append([]any{g}, row3...)
		}
		grid.Set(
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, cpuGauge),
			),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0/2, gpuGauge),
				ui.NewCol(1.0/2, memoryGauge),
			),
			ui.NewRow(1.0/4, equalCols(row3...)...),
			ui.NewRow(1.0/4,
				ui.NewCol(1.0, processList),
			),
		)
	case LayoutPico:
		// Maximum density with 2x2 gauges + sparklines
		grid.Set(
			ui.NewRow(1.0/3, equalCols(gaugeItems(cpuGauge, gpuGauge, memoryGauge)...)...),
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/2, sparklineGroup),
				ui.NewCol(1.0/2, gpuSparklineGroup),
			),
			ui.NewRow(1.0/3,
				ui.NewCol(1.0/3, PowerChart),
				ui.NewCol(1.0/3, NetworkInfo),
				ui.NewCol(1.0/3, modelText),
			),
		)
	}
}

// setMultiGPULayoutGrid lays out one gauge per detected GPU in a two-column
// grid, with the process list across the bottom. When no multi-GPU data is
// available it falls back to the default layout.
func setMultiGPULayoutGrid() {
	n := len(multiGpuGauges)
	if n == 0 {
		setLayoutGrid(LayoutDefault)
		return
	}

	gpuRows := (n + 1) / 2
	totalRows := gpuRows + 1
	rowRatio := 1.0 / float64(totalRows)

	var entries []any
	for i := 0; i < n; i += 2 {
		cols := []any{ui.NewCol(0.5, multiGpuGauges[i])}
		if i+1 < n {
			cols = append(cols, ui.NewCol(0.5, multiGpuGauges[i+1]))
		}
		entries = append(entries, ui.NewRow(rowRatio, cols...))
	}
	entries = append(entries, ui.NewRow(rowRatio, ui.NewCol(1.0, processList)))
	grid.Set(entries...)
}

func setInfoFanLayoutGrid(layoutName string) {
	if layoutName == LayoutFan {
		grid.Set(
			ui.NewRow(0.92,
				ui.NewCol(0.5, fanStatusPanel),
				ui.NewCol(0.5, fanTempPanel),
			),
			ui.NewRow(0.08,
				ui.NewCol(1.0, fanControlPanel),
			),
		)
	} else {
		grid.Set(
			ui.NewRow(1.0,
				ui.NewCol(1.0, infoParagraph),
			),
		)
	}
}

func setHistoryLikeLayoutGrid(layoutName string) {
	switch layoutName {
	case LayoutHistoryFull:
		setHistoryFullLayoutGrid()
	case LayoutGPUMemory:
		setGPUMemoryLayoutGrid()
	default:
		setHistoryLayoutGrid()
	}
}

func setHistoryLayoutGrid() {
	grid.Set(
		ui.NewRow(1.0/3,
			ui.NewCol(1.0, gpuHistoryChart),
		),
		ui.NewRow(1.0/3,
			ui.NewCol(1.0/2, powerHistoryChart),
			ui.NewCol(1.0/2, memoryHistoryChart),
		),
		ui.NewRow(1.0/3,
			ui.NewCol(1.0, processList),
		),
	)
}

func setGPUMemoryLayoutGrid() {
	grid.Set(
		ui.NewRow(1.0/4,
			ui.NewCol(1.0/2, gpuGauge),
			ui.NewCol(1.0/2, memoryGauge),
		),
		ui.NewRow(1.0/4,
			ui.NewCol(1.0/2, gpuSparklineGroup),
			ui.NewCol(1.0/2, memoryHistoryChart),
		),
		ui.NewRow(1.0/4,
			ui.NewCol(1.0, memBWHistoryChart),
		),
		ui.NewRow(1.0/4,
			ui.NewCol(1.0, processList),
		),
	)
}

func setHistoryFullLayoutGrid() {
	grid.Set(
		ui.NewRow(1.0/3,
			ui.NewCol(1.0/2, cpuHistoryChart),
			ui.NewCol(1.0/2, gpuHistoryChart),
		),
		ui.NewRow(1.0/3,
			ui.NewCol(1.0/2, powerHistoryChart),
			ui.NewCol(1.0/2, memoryHistoryChart),
		),
		ui.NewRow(1.0/3,
			ui.NewCol(1.0, processList),
		),
	)
}

func setHistorySoCLayoutGrid() {
	// SoC History layout for ANE/ML workloads (all charts, no process list):
	// Row 1: CPU + GPU
	// Row 2: ANE + SoC Power (rightmost)
	// Row 3 (bottom): Memory BW (left) | Memory Used | SSD Read
	// Rows scaled ×1.25 from the former 0.24/0.24/0.32 to reclaim the height
	// the process list used to occupy.
	// Row 2 holds the ANE history chart only when an Apple Neural Engine is
	// present; otherwise the SoC power chart takes the full width.
	var row2 any
	if hasANE() {
		row2 = ui.NewRow(0.30,
			ui.NewCol(1.0/2, aneHistoryChart),
			ui.NewCol(1.0/2, socPowerHistoryChart),
		)
	} else {
		row2 = ui.NewRow(0.30,
			ui.NewCol(1.0, socPowerHistoryChart),
		)
	}
	grid.Set(
		ui.NewRow(0.30,
			ui.NewCol(1.0/2, cpuHistoryChart),
			ui.NewCol(1.0/2, gpuHistoryChart),
		),
		row2,
		ui.NewRow(0.40,
			ui.NewCol(1.0/3, bandwidthHistoryChart), // Memory BW leftmost
			ui.NewCol(1.0/3, memoryHistoryChart),
			ui.NewCol(1.0/3, ssdReadHistoryChart),
		),
	)

	// Force orange for memory graph specifically in history_soc
	if currentConfig.DefaultLayout == LayoutHistorySoC && memoryHistoryChart != nil {
		memoryHistoryChart.LineColors = []ui.Color{ui.ColorOrange, ui.ColorMagenta}
	}
}
