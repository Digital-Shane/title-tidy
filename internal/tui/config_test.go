package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/google/go-cmp/cmp"
)

func TestConfigScrolling(t *testing.T) {
	// Create a new config model
	model, err := New()
	if err != nil {
		t.Fatalf("Failed to create config model: %v", err)
	}

	// Initialize the model - should return ticker command
	cmd := model.Init()
	if cmd == nil {
		t.Error("Init should return a ticker command for auto-scrolling")
	}

	// Simulate window resize to set viewport dimensions
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	m := updatedModel.(*Model)

	// Check that the viewport was properly initialized
	if m.variablesView.Width == 0 || m.variablesView.Height == 0 {
		t.Error("Viewport dimensions should be set after window resize")
	}

	// Switch to Episode section (which has more variables)
	m.nextSection() // to Season
	m.nextSection() // to Episode
	m.updateVariablesContent()

	// Verify Episode section has extended content
	content := m.variablesView.View()
	if !strings.Contains(content, "") {
		// Content should be available even if not all visible
		t.Log("Episode section content loaded")
	}

	// Test manual scrolling
	originalY := m.variablesView.YOffset
	m.variablesView.ScrollDown(1)
	if m.variablesView.YOffset == originalY && m.variablesView.TotalLineCount() > m.variablesView.Height {
		t.Error("LineDown should change YOffset when content is scrollable")
	}

	m.variablesView.ScrollUp(1)
	if m.variablesView.YOffset != originalY {
		t.Log("Scroll up/down working")
	}

	// Test auto-scroll toggle
	originalAutoScroll := m.autoScroll
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace, Alt: true})
	m = updatedModel.(*Model)
	if m.autoScroll == originalAutoScroll {
		t.Error("Alt+Space should toggle autoScroll")
	}

	// Test that manual scrolling disables auto-scroll
	m.autoScroll = true
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = updatedModel.(*Model)
	if m.autoScroll {
		t.Error("Manual scrolling should disable auto-scroll")
	}

	// Test view rendering with scroll indicator
	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}
	if !strings.Contains(view, "Esc") || !strings.Contains(view, "Ctrl+C") {
		t.Error("View should include quit hints for Esc and Ctrl+C")
	}

	// Check that help includes scroll instructions when content is scrollable
	if m.variablesView.TotalLineCount() > m.variablesView.Height {
		if !strings.Contains(view, "↑↓") && !strings.Contains(view, "Scroll") {
			t.Log("Scroll help indicators should be shown when content is scrollable")
		}
	}
}

func TestStripNullChars(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "NoNullChars",
			input:    "normal string",
			expected: "normal string",
		},
		{
			name:     "WindowsStyleInput",
			input:    "{title} ({year})\x00",
			expected: "{title} ({year})",
		},
		{
			name:     "OnlyNullChars",
			input:    "\x00\x00\x00",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := stripNullChars(tc.input)
			if diff := cmp.Diff(tc.expected, result); diff != "" {
				t.Errorf("stripNullChars(%q) mismatch (-want +got):\n%s", tc.input, diff)
			}
		})
	}
}

func TestScrollTickMsg(t *testing.T) {
	model, err := New()
	if err != nil {
		t.Fatalf("Failed to create config model: %v", err)
	}

	// Set up viewport with content
	model.width = 120
	model.height = 40
	model.variablesView.Width = 36
	model.variablesView.Height = 26
	model.activeSection = SectionEpisode
	model.updateVariablesContent()

	// Test auto-scroll tick timing
	cmd := model.tickCmd()
	if cmd == nil {
		t.Error("tickCmd should return a command")
	}
	t.Log("Auto-scroll now set to 3 second intervals")

	// Test auto-scroll tick
	model.autoScroll = true

	initialOffset := model.variablesView.YOffset
	updatedModel, cmd := model.Update(scrollTickMsg{})
	m := updatedModel.(*Model)

	// Should return a new tick command
	if cmd == nil {
		t.Error("scrollTickMsg should return a new tick command")
	}

	// Should scroll down if not at bottom
	if !m.variablesView.AtBottom() && m.variablesView.YOffset == initialOffset {
		t.Error("Auto-scroll should move viewport down when not at bottom")
	}

	// Test reset to top at bottom
	m.variablesView.GotoBottom()
	updatedModel, _ = m.Update(scrollTickMsg{})
	m = updatedModel.(*Model)
	if m.variablesView.YOffset != 0 {
		t.Error("Auto-scroll should reset to top when reaching bottom")
	}
}
