package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
)

// StepTracker tracks and prints sequential step progress with checkmarks.
type StepTracker struct {
	mu      sync.Mutex
	out     io.Writer
	quiet   bool
	verbose bool
	current string
	active  bool
	stopCh  chan struct{}
}

// NewStepTracker creates a StepTracker printing to os.Stdout.
func NewStepTracker(quiet, verbose bool) *StepTracker {
	return &StepTracker{
		out:     os.Stdout,
		quiet:   quiet,
		verbose: verbose,
	}
}

// Start begins a named step and optionally shows an inline progress indicator.
func (st *StepTracker) Start(msg string) {
	if st.quiet {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	st.current = msg
	st.active = true

	if st.verbose {
		fmt.Fprintf(st.out, "%s %s...\n", FormatInfo("Running:"), msg)
	} else {
		fmt.Fprintf(st.out, "  %s %s...", HighlightStyle.Render(IconArrow), msg)
	}
}

// Success marks the current step as completed successfully with a green checkmark.
func (st *StepTracker) Success(msg string) {
	if st.quiet {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	targetMsg := msg
	if targetMsg == "" {
		targetMsg = st.current
	}

	if st.verbose {
		fmt.Fprintf(st.out, "%s\n", FormatSuccess(targetMsg))
	} else {
		// Overwrite the current line or append checkmark
		fmt.Fprintf(st.out, "\r  %s %s\n", SuccessStyle.Render(IconCheck), targetMsg)
	}
	st.active = false
}

// Warn marks the step with a warning icon.
func (st *StepTracker) Warn(msg string) {
	if st.quiet {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	targetMsg := msg
	if targetMsg == "" {
		targetMsg = st.current
	}

	if st.verbose {
		fmt.Fprintf(st.out, "%s\n", FormatWarning(targetMsg))
	} else {
		fmt.Fprintf(st.out, "\r  %s %s\n", WarningStyle.Render(IconWarning), targetMsg)
	}
	st.active = false
}

// Fail marks the step as failed with a red cross.
func (st *StepTracker) Fail(msg string) {
	if st.quiet {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()

	targetMsg := msg
	if targetMsg == "" {
		targetMsg = st.current
	}

	if st.verbose {
		fmt.Fprintf(st.out, "%s\n", FormatError(targetMsg))
	} else {
		fmt.Fprintf(st.out, "\r  %s %s\n", DangerStyle.Render(IconCross), targetMsg)
	}
	st.active = false
}

// Info prints an informational bullet point.
func (st *StepTracker) Info(msg string) {
	if st.quiet {
		return
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	fmt.Fprintf(st.out, "    %s %s\n", MutedStyle.Render(IconBullet), msg)
}

// PrintProgressBar renders a text progress bar for downloads.
func FormatProgressBar(percent float64, width int) string {
	if width < 10 {
		width = 20
	}
	filled := int(float64(width) * (percent / 100.0))
	if filled > width {
		filled = width
	}
	if filled < 0 {
		filled = 0
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %5.1f%%", bar, percent)
}
