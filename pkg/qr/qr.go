// Package qr provides QR code generation and styled terminal rendering.
package qr

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/skip2/go-qrcode"
)

// Renderer handles QR code rendering with styled terminal output.
type Renderer struct {
	quiet bool
}

// NewRenderer creates a new QR code renderer.
func NewRenderer(quiet bool) *Renderer {
	return &Renderer{quiet: quiet}
}

// Styles for terminal output using Lipgloss.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("205")).
			MarginBottom(1)

	urlStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("42")).
			Background(lipgloss.Color("235")).
			Padding(0, 1)

	qrStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("255")).
		Background(lipgloss.Color("0"))

	infoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244")).
			Italic(true).
			MarginTop(1)

	errorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("196"))

	successStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("82"))

	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("63")).
			Padding(1, 2).
			Align(lipgloss.Center)
)

// GenerateQRString generates a QR code as a string for terminal display.
// Uses Unicode block characters for compact display.
func GenerateQRString(url string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}

	// Get the bitmap representation
	bitmap := qr.Bitmap()
	size := len(bitmap)

	var sb strings.Builder

	// Use Unicode half blocks for compact display
	// Each character represents 2 vertical pixels
	for y := 0; y < size; y += 2 {
		for x := 0; x < size; x++ {
			upper := bitmap[y][x]
			lower := false
			if y+1 < size {
				lower = bitmap[y+1][x]
			}

			// Unicode block characters:
			// ▀ (upper half) - upper is black, lower is white
			// ▄ (lower half) - upper is white, lower is black
			// █ (full block) - both are black
			// ' ' (space) - both are white
			switch {
			case upper && lower:
				sb.WriteString("█")
			case upper && !lower:
				sb.WriteString("▀")
			case !upper && lower:
				sb.WriteString("▄")
			default:
				sb.WriteString(" ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// RenderOutput renders the complete styled output with QR code and URL.
func (r *Renderer) RenderOutput(url string, isPublic bool) error {
	qrString, err := GenerateQRString(url)
	if err != nil {
		return err
	}

	// In quiet mode, only output the URL and QR
	if r.quiet {
		// Minimal output
		styledURL := urlStyle.Render(url)
		styledQR := qrStyle.Render(qrString)

		output := lipgloss.JoinVertical(lipgloss.Center,
			styledQR,
			styledURL,
		)

		// Center in terminal
		centeredOutput := lipgloss.Place(
			80, 0, // width, height (0 = auto)
			lipgloss.Center, lipgloss.Center,
			output,
		)

		println(centeredOutput)
		return nil
	}

	// Full styled output
	var title string
	if isPublic {
		title = titleStyle.Render("🌐 Public URL (via SSH tunnel)")
	} else {
		title = titleStyle.Render("📡 Local Network URL")
	}

	styledURL := urlStyle.Render(url)
	styledQR := qrStyle.Render(qrString)

	info := infoStyle.Render("Scan the QR code or visit the URL above")

	content := lipgloss.JoinVertical(lipgloss.Center,
		title,
		styledQR,
		styledURL,
		info,
	)

	boxedContent := boxStyle.Render(content)

	// Center in terminal
	centeredOutput := lipgloss.Place(
		80, 0,
		lipgloss.Center, lipgloss.Center,
		boxedContent,
	)

	println(centeredOutput)
	return nil
}

// PrintError prints a styled error message.
func (r *Renderer) PrintError(message string) {
	if r.quiet {
		return
	}
	styled := errorStyle.Render("✗ Error: " + message)
	println(styled)
}

// PrintSuccess prints a styled success message.
func (r *Renderer) PrintSuccess(message string) {
	if r.quiet {
		return
	}
	styled := successStyle.Render("✓ " + message)
	println(styled)
}

// PrintInfo prints a styled info message.
func (r *Renderer) PrintInfo(message string) {
	if r.quiet {
		return
	}
	styled := infoStyle.Render("ℹ " + message)
	println(styled)
}
