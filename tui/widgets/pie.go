package widgets

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// PieSlice represents a data segment
type PieSlice struct {
	Label   string
	Value   float64
	Color   lipgloss.Color
	Percent float64
}

// BlockPieChart renders a high-resolution pie chart using block characters
type BlockPieChart struct {
	Slices []PieSlice
	Width  int
	Height int
	Radius int
}

// NewBlockPieChart creates a new chart
func NewBlockPieChart(slices []PieSlice) *BlockPieChart {
	var total float64
	for i := range slices {
		total += slices[i].Value
	}
	for i := range slices {
		if total > 0 {
			slices[i].Percent = slices[i].Value / total * 100
		}
	}

	return &BlockPieChart{
		Slices: slices,
		Width:  40,
		Height: 20,
		Radius: 9, // Radius in cells
	}
}

// Render generates the chart string
func (p *BlockPieChart) Render() string {
	if len(p.Slices) == 0 {
		return "No data"
	}

	// Calculate total for percentage calculation
	var total float64
	for _, s := range p.Slices {
		total += s.Value
	}
	if total == 0 {
		return "No data"
	}

	// Create the grid
	gridWidth := p.Width
	gridHeight := p.Height
	grid := make([][]rune, gridHeight)
	for y := 0; y < gridHeight; y++ {
		grid[y] = make([]rune, gridWidth)
		for x := 0; x < gridWidth; x++ {
			grid[y][x] = ' '
		}
	}

	// Center of the pie chart
	centerX := float64(gridWidth / 2)
	centerY := float64(gridHeight / 2)

	// Adjust radius based on the smaller dimension to fit in the box
	maxRadius := math.Min(centerX, centerY) - 1
	if float64(p.Radius) > maxRadius {
		p.Radius = int(maxRadius)
	}

	// Calculate start and end angles for each slice
	startAngle := -math.Pi / 2 // Start from top

	for _, slice := range p.Slices {
		// Calculate the angle for this slice
		angle := (slice.Value / total) * 2 * math.Pi
		endAngle := startAngle + angle

		// Draw the slice
		for angleStep := 0.0; angleStep < angle; angleStep += 0.1 {
			// Adjust step size for better resolution
			currentAngle := startAngle + angleStep

			// Draw from center to edge
			for r := 0; r <= p.Radius; r++ {
				rx := int(centerX + math.Cos(currentAngle)*float64(r))
				ry := int(centerY + math.Sin(currentAngle)*float64(r)*0.8)

				// Check bounds
				if rx >= 0 && rx < gridWidth && ry >= 0 && ry < gridHeight {
					grid[ry][rx] = '●' // Using a bullet character for better appearance
				}
			}
		}

		startAngle = endAngle
	}

	// Convert grid to string with color
	var sb strings.Builder
	for y := 0; y < gridHeight; y++ {
		for x := 0; x < gridWidth; x++ {
			// Determine which slice this cell belongs to
			cx := float64(x) - centerX
			cy := (float64(y) - centerY) / 0.8 // Compensate for vertical compression
			distance := math.Sqrt(cx*cx + cy*cy)

			if distance <= float64(p.Radius) && grid[y][x] != ' ' {
				// Find which slice this pixel belongs to
				angle := math.Atan2(cy, cx)
				if angle < 0 {
					angle += 2 * math.Pi
				}
				// Normalize start angle to 0-2π
				normalizedStartAngle := -math.Pi / 2
				if normalizedStartAngle < 0 {
					normalizedStartAngle += 2 * math.Pi
				}

				// Find which slice this point belongs to
				currentAngle := angle
				if currentAngle < normalizedStartAngle {
					currentAngle += 2 * math.Pi
				}

				sliceIndex := -1
				currentStart := normalizedStartAngle
				for i, slice := range p.Slices {
					sectorAngle := (slice.Value / total) * 2 * math.Pi
					if currentAngle >= currentStart && currentAngle < currentStart+sectorAngle {
						sliceIndex = i
						break
					}
					currentStart += sectorAngle
				}

				if sliceIndex >= 0 && sliceIndex < len(p.Slices) {
					style := lipgloss.NewStyle().Foreground(p.Slices[sliceIndex].Color)
					sb.WriteString(style.Render(string('●')))
				} else {
					sb.WriteString(string(grid[y][x]))
				}
			} else {
				sb.WriteString(string(grid[y][x]))
			}
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// EnhancedPieChartWithLegend creates a pie chart with a professional legend
func EnhancedPieChartWithLegend(slices []PieSlice, width, height int) string {
	if len(slices) == 0 {
		return "No data"
	}

	// Create the pie chart
	chart := NewBlockPieChart(slices)
	chart.Width = width / 2
	chart.Height = height
	chart.Radius = int(math.Min(float64(chart.Width/2), float64(chart.Height/2))) - 1

	// Create the chart string
	chartStr := chart.Render()

	// Create legend
	var legendLines []string
	for i, s := range slices {
		if i == 0 {
			legendLines = append(legendLines, " Legend")
		}
		colorBox := lipgloss.NewStyle().Foreground(s.Color).Render("██")
		name := s.Label
		if len(name) > 15 {
			name = name[:12] + "..."
		}
		sizeStr := formatBytes(s.Value)
		percentStr := fmt.Sprintf("%.1f%%", s.Percent)
		line := fmt.Sprintf("%s %-15s %8s (%s)", colorBox, name, sizeStr, percentStr)
		legendLines = append(legendLines, line)
	}

	legendStr := strings.Join(legendLines, "\n")

	// Combine chart and legend horizontally
	chartLines := strings.Split(strings.TrimRight(chartStr, "\n"), "\n")
	legendLinesSplit := strings.Split(legendStr, "\n")

	// Determine max height
	maxHeight := len(chartLines)
	if len(legendLinesSplit) > maxHeight {
		maxHeight = len(legendLinesSplit)
	}

	var result strings.Builder
	for i := 0; i < maxHeight; i++ {
		chartLine := ""
		if i < len(chartLines) {
			chartLine = chartLines[i]
		}

		legendLine := ""
		if i < len(legendLinesSplit) {
			legendLine = legendLinesSplit[i]
		}

		result.WriteString(fmt.Sprintf("%s    %s\n", chartLine, legendLine))
	}

	return result.String()
}

// Helper function to format bytes
func formatBytes(bytes float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	unit := 0
	val := bytes
	for val >= 1024 && unit < len(units)-1 {
		val /= 1024
		unit++
	}
	if unit == 0 {
		return fmt.Sprintf("%.0f%s", val, units[unit])
	}
	return fmt.Sprintf("%.1f%s", val, units[unit])
}
