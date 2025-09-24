package config

import (
	"bytes"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/google/go-cmp/cmp"
)

func newConfigTestModel(t *testing.T) *teatest.TestModel {
	t.Helper()

	t.Setenv("HOME", t.TempDir())

	model, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	model.tmdbValidate = func(apiKey string) tea.Cmd {
		return func() tea.Msg {
			if apiKey == "" {
				return tmdbValidationMsg{apiKey: apiKey, valid: false}
			}
			return tmdbValidationMsg{apiKey: apiKey, valid: !strings.Contains(apiKey, "invalid")}
		}
	}
	model.tmdbDebounce = func(apiKey string) tea.Cmd {
		return func() tea.Msg {
			return tmdbValidateCmd{apiKey: apiKey}
		}
	}
	model.omdbValidate = func(apiKey string) tea.Cmd {
		return func() tea.Msg {
			if len(apiKey) < 4 {
				return omdbValidationMsg{apiKey: apiKey, valid: false}
			}
			return omdbValidationMsg{apiKey: apiKey, valid: !strings.Contains(apiKey, "invalid")}
		}
	}
	model.omdbDebounce = func(apiKey string) tea.Cmd {
		return func() tea.Msg {
			return omdbValidateCmd{apiKey: apiKey}
		}
	}

	tm := teatest.NewTestModel(t, model, teatest.WithInitialTermSize(100, 36))
	t.Cleanup(func() {
		_ = tm.Quit()
	})

	return tm
}

func waitForOutput(t *testing.T, tm *teatest.TestModel, contains string) {
	t.Helper()

	teatest.WaitFor(t, tm.Output(), func(b []byte) bool {
		return bytes.Contains(b, []byte(contains))
	}, teatest.WithDuration(2*time.Second), teatest.WithCheckInterval(10*time.Millisecond))
}

func press(tm *teatest.TestModel, key tea.KeyType, edits ...func(*tea.KeyMsg)) {
	msg := tea.KeyMsg{Type: key}
	for _, edit := range edits {
		edit(&msg)
	}
	tm.Send(msg)
}

func withAlt(msg *tea.KeyMsg) { msg.Alt = true }

func backspaceN(tm *teatest.TestModel, n int) {
	for i := 0; i < n; i++ {
		press(tm, tea.KeyBackspace)
	}
}

func finalConfigModel(t *testing.T, tm *teatest.TestModel) *Model {
	t.Helper()

	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	model, ok := final.(*Model)
	if !ok {
		t.Fatalf("Final model was %T, want *Model", final)
	}
	return model
}

func TestConfigTUISectionNavigation(t *testing.T) {
	tm := newConfigTestModel(t)

	waitForOutput(t, tm, "[ Show Folder ]")

	tabs := []string{"[ Season Folder ]", "[ Episode ]", "[ Movie ]", "[ Logging ]", "[ Providers ]"}
	for _, label := range tabs {
		press(tm, tea.KeyTab)
		waitForOutput(t, tm, label)
	}

	press(tm, tea.KeyTab)
	waitForOutput(t, tm, "[ Show Folder ]")

	press(tm, tea.KeyShiftTab)
	waitForOutput(t, tm, "[ Providers ]")

	press(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
}

func TestConfigTUITemplateEditingKeys(t *testing.T) {
	tm := newConfigTestModel(t)

	waitForOutput(t, tm, "[ Show Folder ]")

	press(tm, tea.KeyEnd)
	const defaultShow = "{title} ({year})"
	backspaceN(tm, len(defaultShow))
	tm.Type("Test")
	press(tm, tea.KeySpace)
	press(tm, tea.KeyLeft)
	tm.Type("X")
	press(tm, tea.KeyBackspace)
	press(tm, tea.KeyRight)
	press(tm, tea.KeyHome)
	tm.Type("B")
	press(tm, tea.KeyEnd)
	tm.Type("Z")

	press(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	model := finalConfigModel(t, tm)

	want := "BTest Z"
	if diff := cmp.Diff(want, model.inputs[SectionShowFolder]); diff != "" {
		t.Errorf("inputs[ShowFolder] diff (-want +got):\n%s", diff)
	}

	if got := model.cursorPos[SectionShowFolder]; got != len(want) {
		t.Errorf("cursorPos = %d, want %d", got, len(want))
	}
}

func TestConfigTUILoggingKeys(t *testing.T) {
	tm := newConfigTestModel(t)
	waitForOutput(t, tm, "[ Show Folder ]")

	for range []int{0, 1, 2, 3} {
		press(tm, tea.KeyTab)
	}
	waitForOutput(t, tm, "Logging Configuration")

	press(tm, tea.KeySpace)
	press(tm, tea.KeyDown)
	tm.Type("90")
	press(tm, tea.KeySpace)
	press(tm, tea.KeyUp)
	press(tm, tea.KeyEnter)
	press(tm, tea.KeyDown)
	press(tm, tea.KeyBackspace)
	tm.Type("5")
	press(tm, tea.KeySpace)
	tm.Type("7")

	press(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	model := finalConfigModel(t, tm)

	if !model.loggingEnabled {
		t.Error("loggingEnabled = false, want true")
	}

	if diff := cmp.Diff("357", model.loggingRetention); diff != "" {
		t.Errorf("loggingRetention diff (-want +got):\n%s", diff)
	}

	if got := model.loggingSubfocus; got != 1 {
		t.Errorf("loggingSubfocus = %d, want 1", got)
	}
}

func TestConfigTUIScrollingKeys(t *testing.T) {
	tm := newConfigTestModel(t)
	waitForOutput(t, tm, "[ Show Folder ]")

	press(tm, tea.KeyTab)
	press(tm, tea.KeyTab)
	waitForOutput(t, tm, "[ Episode ]")

	press(tm, tea.KeyPgDown)
	press(tm, tea.KeyPgUp)
	press(tm, tea.KeyDown)
	press(tm, tea.KeyDown)
	press(tm, tea.KeyUp)
	press(tm, tea.KeySpace, withAlt)

	press(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	model := finalConfigModel(t, tm)

	if !model.autoScroll {
		t.Error("autoScroll = false, want true after Alt+Space")
	}

	if model.variablesView.YOffset == 0 {
		t.Error("YOffset = 0, want non-zero after manual scroll")
	}
}

func TestConfigTUISaveAndReset(t *testing.T) {
	tm := newConfigTestModel(t)
	waitForOutput(t, tm, "[ Show Folder ]")

	press(tm, tea.KeyEnd)
	backspaceN(tm, len("{title} ({year})"))
	tm.Type("Alpha")
	press(tm, tea.KeyCtrlS)
	waitForOutput(t, tm, "Configuration saved!")

	press(tm, tea.KeyEnd)
	backspaceN(tm, len("Alpha"))
	tm.Type("Beta")
	press(tm, tea.KeyCtrlR)

	press(tm, tea.KeyEsc)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	model := finalConfigModel(t, tm)

	if diff := cmp.Diff("Alpha", model.inputs[SectionShowFolder]); diff != "" {
		t.Errorf("inputs[ShowFolder] diff (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff("Reset to saved values", model.saveStatus); diff != "" {
		t.Errorf("saveStatus diff (-want +got):\n%s", diff)
	}
}

func TestConfigTUIProvidersTMDB(t *testing.T) {
	tm := newConfigTestModel(t)
	waitForOutput(t, tm, "[ Show Folder ]")

	for range []int{0, 1, 2, 3, 4} {
		press(tm, tea.KeyTab)
	}
	waitForOutput(t, tm, "Provider Controls")

	press(tm, tea.KeyRight)
	press(tm, tea.KeyRight)
	press(tm, tea.KeyRight)

	press(tm, tea.KeySpace)
	press(tm, tea.KeyDown)
	tm.Type("valid123")
	press(tm, tea.KeyBackspace)
	tm.Type("X")
	press(tm, tea.KeyDown)
	for i := 0; i < 5; i++ {
		press(tm, tea.KeyBackspace)
	}
	tm.Type("es-ES1")
	press(tm, tea.KeyUp)
	press(tm, tea.KeyUp)
	press(tm, tea.KeyEnter)

	press(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	model := finalConfigModel(t, tm)

	if model.tmdbEnabled {
		t.Error("tmdbEnabled = true, want false after toggle off")
	}

	if diff := cmp.Diff("valid12X", model.tmdbAPIKey); diff != "" {
		t.Errorf("tmdbAPIKey diff (-want +got):\n%s", diff)
	}

	if diff := cmp.Diff("es-ES", model.tmdbLanguage); diff != "" {
		t.Errorf("tmdbLanguage diff (-want +got):\n%s", diff)
	}

	if got := model.tmdbSubfocus; got != 0 {
		t.Errorf("tmdbSubfocus = %d, want 0", got)
	}

	if diff := cmp.Diff("", model.tmdbValidation); diff != "" {
		t.Errorf("tmdbValidation diff (-want +got):\n%s", diff)
	}
}

func TestConfigTUIProvidersOMDB(t *testing.T) {
	tm := newConfigTestModel(t)
	waitForOutput(t, tm, "[ Show Folder ]")

	for range []int{0, 1, 2, 3, 4} {
		press(tm, tea.KeyTab)
	}
	waitForOutput(t, tm, "Provider Controls")

	press(tm, tea.KeyRight)
	press(tm, tea.KeyRight)

	press(tm, tea.KeySpace)
	press(tm, tea.KeyDown)
	tm.Type("abcd")
	press(tm, tea.KeyBackspace)
	tm.Type("Z9")
	press(tm, tea.KeyUp)
	press(tm, tea.KeyEnter)

	press(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	model := finalConfigModel(t, tm)

	if model.omdbEnabled {
		t.Error("omdbEnabled = true, want false after toggle off")
	}

	if diff := cmp.Diff("abcZ9", model.omdbAPIKey); diff != "" {
		t.Errorf("omdbAPIKey diff (-want +got):\n%s", diff)
	}

	if got := model.omdbSubfocus; got != 0 {
		t.Errorf("omdbSubfocus = %d, want 0", got)
	}

}

func TestConfigTUIProvidersSharedAndFFProbe(t *testing.T) {
	tm := newConfigTestModel(t)
	waitForOutput(t, tm, "[ Show Folder ]")

	for range []int{0, 1, 2, 3, 4} {
		press(tm, tea.KeyTab)
	}
	waitForOutput(t, tm, "Provider Controls")

	press(tm, tea.KeyBackspace)
	tm.Type("23")
	tm.Type("a")
	press(tm, tea.KeySpace)
	press(tm, tea.KeyRight)
	press(tm, tea.KeySpace)
	press(tm, tea.KeyEnter)
	press(tm, tea.KeySpace)

	press(tm, tea.KeyCtrlC)
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))

	model := tm.FinalModel(t).(*Model)

	if diff := cmp.Diff("123", model.workerCount); diff != "" {
		t.Errorf("workerCount diff (-want +got):\n%s", diff)
	}

	if !model.ffprobeEnabled {
		t.Error("ffprobeEnabled = false, want true")
	}

	if diff := cmp.Diff(providerColumnFFProbe, model.providerColumnFocus); diff != "" {
		t.Errorf("providerColumnFocus diff (-want +got):\n%s", diff)
	}
}
