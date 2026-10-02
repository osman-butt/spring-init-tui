package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

type fakeBackend struct {
	metadata initializr.Metadata
	err      error
}

func (f fakeBackend) Metadata(context.Context) (initializr.Metadata, error) {
	return f.metadata, f.err
}

var testMetadata = initializr.Metadata{
	JavaVersions:       []string{"25", "21", "17"},
	DefaultJavaVersion: "17",
	Dependencies: []initializr.Dependency{
		{ID: "web", Name: "Spring Web", Group: "Web"},
		{ID: "data-jpa", Name: "Spring Data JPA", Group: "SQL"},
	},
}

func press(s string) tea.KeyPressMsg {
	switch s {
	case "ctrl+c":
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

// step runs Update and asserts the returned model type.
func step(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	nm, ok := next.(Model)
	if !ok {
		t.Fatalf("Update returned %T, want Model", next)
	}
	return nm, cmd
}

// keys presses each key in turn. A single-character key is typed as text.
func keys(t *testing.T, m Model, keys ...string) Model {
	t.Helper()
	for _, k := range keys {
		m, _ = step(t, m, press(k))
	}
	return m
}

func typeText(t *testing.T, m Model, text string) Model {
	t.Helper()
	for _, r := range text {
		m, _ = step(t, m, press(string(r)))
	}
	return m
}

// loaded returns a model on the first screen after metadata has arrived, in
// a directory where no project exists yet.
func loaded(t *testing.T) Model {
	t.Helper()
	m := New(fakeBackend{metadata: testMetadata})
	m.exists = func(string) bool { return false }
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, metadataLoadedMsg{testMetadata})
	return m
}

func assertQuits(t *testing.T, cmd tea.Cmd) {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a quit command, got none")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("expected the command to quit")
	}
}

func TestLoadCmd(t *testing.T) {
	msg := New(fakeBackend{metadata: testMetadata}).loadCmd()()
	got, ok := msg.(metadataLoadedMsg)
	if !ok || len(got.metadata.Dependencies) != 2 {
		t.Errorf("loadCmd() = %#v, want metadataLoadedMsg with 2 dependencies", msg)
	}

	msg = New(fakeBackend{err: errors.New("boom")}).loadCmd()()
	if failed, ok := msg.(loadFailedMsg); !ok || failed.err.Error() != "boom" {
		t.Errorf("loadCmd() = %#v, want loadFailedMsg{boom}", msg)
	}
}

func TestLoadingShowsSpinnerText(t *testing.T) {
	m := New(fakeBackend{})
	if m.screen != screenLoading {
		t.Fatalf("screen = %v, want loading", m.screen)
	}
	out := m.View().Content
	for _, want := range []string{"SPRING INITIALIZR", "Build. Configure. Generate.", "Fetching metadata"} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing %q:\n%s", want, out)
		}
	}
}

func TestMetadataLoadedShowsNameScreen(t *testing.T) {
	m := New(fakeBackend{})
	m, cmd := step(t, m, metadataLoadedMsg{testMetadata})
	if m.screen != screenName {
		t.Fatalf("screen = %v, want name", m.screen)
	}
	if !m.input.Focused() || cmd == nil {
		t.Error("the name input should be focused and return its blink command")
	}
	if !strings.Contains(m.View().Content, "Project") {
		t.Errorf("name view is missing its header:\n%s", m.View().Content)
	}
}

func TestLoadFailureCanBeRetried(t *testing.T) {
	m := New(fakeBackend{metadata: testMetadata})
	m, _ = step(t, m, loadFailedMsg{errors.New("boom")})
	if m.screen != screenError {
		t.Fatalf("screen = %v, want error", m.screen)
	}
	out := m.View().Content
	if !strings.Contains(out, "boom") || !strings.Contains(out, "retry") {
		t.Errorf("error view should show the message and the retry key:\n%s", out)
	}

	m, cmd := step(t, m, press("r"))
	if m.screen != screenLoading || m.err != nil {
		t.Fatalf("after retry: screen = %v, err = %v", m.screen, m.err)
	}
	if cmd == nil {
		t.Fatal("retry returned no command")
	}
	if strings.Contains(m.View().Content, "retry") {
		t.Error("retry key should not be offered while loading")
	}
}

func TestRetryIsIgnoredWhileLoading(t *testing.T) {
	m := New(fakeBackend{})
	m, _ = step(t, m, press("r"))
	if m.screen != screenLoading {
		t.Errorf("screen = %v, want loading", m.screen)
	}
}

func TestQuit(t *testing.T) {
	m := New(fakeBackend{})
	m, cmd := step(t, m, press("q"))
	assertQuits(t, cmd)
	if m.ctx.Err() == nil {
		t.Error("quitting should cancel in-flight work")
	}
	if out := m.View().Content; out != m.bannerView()+"\n" {
		t.Errorf("view after quitting should be the banner only, got:\n%s", out)
	}
	if m.Err() != nil {
		t.Errorf("Err() = %v, want nil", m.Err())
	}
	if m.Interrupted() {
		t.Error("Interrupted() = true after q")
	}
}

func TestQuitFromErrorKeepsTheError(t *testing.T) {
	m := New(fakeBackend{})
	m, _ = step(t, m, loadFailedMsg{errors.New("boom")})
	m, _ = step(t, m, press("q"))

	if m.Err() == nil {
		t.Error("Err() = nil, want the load error")
	}
	out := m.View().Content
	if !strings.Contains(out, "boom") {
		t.Errorf("final view should keep the error:\n%s", out)
	}
	if strings.Contains(out, "retry") {
		t.Errorf("final view should not show the help footer:\n%s", out)
	}
}

func TestCtrlCInterrupts(t *testing.T) {
	m := New(fakeBackend{})
	m, _ = step(t, m, loadFailedMsg{errors.New("boom")})
	m, cmd := step(t, m, press("ctrl+c"))
	assertQuits(t, cmd)
	if !m.Interrupted() {
		t.Error("Interrupted() = false after ctrl+c")
	}
	if m.ctx.Err() == nil {
		t.Error("ctrl+c should cancel in-flight work")
	}
	if out := m.View().Content; out != m.bannerView()+"\n" {
		t.Errorf("view after ctrl+c should be the banner only, got:\n%s", out)
	}
}

func TestNameValidation(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		exists  bool
		wantErr string
	}{
		{name: "empty", input: "", wantErr: "enter a project name"},
		{name: "only spaces", input: "   ", wantErr: "enter a project name"},
		{name: "path separator", input: "a/b", wantErr: "use letters, digits"},
		{name: "parent directory", input: "..", wantErr: "use letters, digits"},
		{name: "space inside", input: "my app", wantErr: "use letters, digits"},
		{name: "already exists", input: "demo", exists: true, wantErr: `"demo" already exists`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := loaded(t)
			m.exists = func(string) bool { return tt.exists }
			m = typeText(t, m, tt.input)
			m = keys(t, m, "enter")

			if m.screen != screenName {
				t.Fatalf("screen = %v, want to stay on name", m.screen)
			}
			if out := m.View().Content; !strings.Contains(out, tt.wantErr) {
				t.Errorf("view should explain the problem (%q):\n%s", tt.wantErr, out)
			}
		})
	}
}

func TestNameErrorClearsWhenTyping(t *testing.T) {
	m := keys(t, loaded(t), "enter")
	if m.nameErr == nil {
		t.Fatal("expected a validation error")
	}
	m = typeText(t, m, "d")
	if m.nameErr != nil {
		t.Errorf("nameErr = %v, want it cleared after typing", m.nameErr)
	}
}

func TestNameAcceptsLettersThatAreShortcutsElsewhere(t *testing.T) {
	m := typeText(t, loaded(t), "qjkr")
	if m.screen != screenName || m.input.Value() != "qjkr" {
		t.Errorf("screen = %v, input = %q; want name screen with %q", m.screen, m.input.Value(), "qjkr")
	}
	if out := m.View().Content; strings.Contains(out, "q quit") {
		t.Errorf("q should not be offered as quit while typing:\n%s", out)
	}
}

func TestNameIsTrimmed(t *testing.T) {
	m := typeText(t, loaded(t), " my-app_1.0 ")
	m = keys(t, m, "enter")
	if m.screen != screenJava || m.name != "my-app_1.0" {
		t.Errorf("screen = %v, name = %q; want java screen with %q", m.screen, m.name, "my-app_1.0")
	}
}

func TestJavaVersionSelection(t *testing.T) {
	m := typeText(t, loaded(t), "demo")
	m = keys(t, m, "enter")
	if m.screen != screenJava {
		t.Fatalf("screen = %v, want java", m.screen)
	}
	if got := m.javaVersion(); got != "17" {
		t.Errorf("preselected Java version = %q, want the default 17", got)
	}

	m = keys(t, m, "down") // already on the last entry
	if got := m.javaVersion(); got != "17" {
		t.Errorf("after down on last entry: %q, want 17", got)
	}
	m = keys(t, m, "up")
	if got := m.javaVersion(); got != "21" {
		t.Errorf("after up: %q, want 21", got)
	}
	m = keys(t, m, "k", "k", "k") // stops at the first entry
	if got := m.javaVersion(); got != "25" {
		t.Errorf("after k k k: %q, want 25", got)
	}
	m = keys(t, m, "j")
	if got := m.javaVersion(); got != "21" {
		t.Errorf("after j: %q, want 21", got)
	}
	if out := m.View().Content; !strings.Contains(out, "> 21") {
		t.Errorf("java view should mark 21 as selected:\n%s", out)
	}

	m = keys(t, m, "enter")
	if m.screen != screenSummary {
		t.Fatalf("screen = %v, want summary", m.screen)
	}
	out := m.View().Content
	if !strings.Contains(out, "demo") || !strings.Contains(out, "21") {
		t.Errorf("summary should show the answers:\n%s", out)
	}
}

func TestJavaDefaultFallsBackToFirstVersion(t *testing.T) {
	m := New(fakeBackend{})
	m, _ = step(t, m, metadataLoadedMsg{initializr.Metadata{
		JavaVersions:       []string{"25", "21"},
		DefaultJavaVersion: "8",
	}})
	if got := m.javaVersion(); got != "25" {
		t.Errorf("javaVersion() = %q, want the first version 25", got)
	}
}

func TestBackNavigation(t *testing.T) {
	m := typeText(t, loaded(t), "demo")
	m = keys(t, m, "enter", "up", "enter")
	if m.screen != screenSummary {
		t.Fatalf("screen = %v, want summary", m.screen)
	}

	m = keys(t, m, "esc")
	if m.screen != screenJava || m.javaVersion() != "21" {
		t.Fatalf("esc on summary: screen = %v, java = %q; want java screen with 21", m.screen, m.javaVersion())
	}

	m, cmd := step(t, m, press("esc"))
	if m.screen != screenName || m.input.Value() != "demo" {
		t.Fatalf("esc on java: screen = %v, input = %q; want name screen with demo", m.screen, m.input.Value())
	}
	if !m.input.Focused() || cmd == nil {
		t.Error("the name input should be focused again")
	}

	_, cmd = step(t, m, press("esc"))
	assertQuits(t, cmd)
}

func TestQuitFromFormScreens(t *testing.T) {
	m := typeText(t, loaded(t), "demo")
	m = keys(t, m, "enter")
	_, cmd := step(t, m, press("q"))
	assertQuits(t, cmd)

	m = keys(t, m, "enter")
	_, cmd = step(t, m, press("q"))
	assertQuits(t, cmd)
}

func TestViewFitsWidth(t *testing.T) {
	longErr := errors.New(strings.Repeat("something went wrong ", 10))
	longName := strings.Repeat("a", 64)

	for _, width := range []int{120, 80, 40, 24} {
		check := func(screen string, m Model) {
			t.Helper()
			if w := lipgloss.Width(m.View().Content); w > width {
				t.Errorf("%s view is %d wide at width %d", screen, w, width)
			}
		}

		m := New(fakeBackend{})
		m.exists = func(string) bool { return false }
		m, _ = step(t, m, tea.WindowSizeMsg{Width: width, Height: 24})
		check("loading", m)

		failed, _ := step(t, m, loadFailedMsg{longErr})
		check("error", failed)

		m, _ = step(t, m, metadataLoadedMsg{testMetadata})
		check("empty name", m)
		check("invalid name", keys(t, typeText(t, m, "not valid!"), "enter"))

		m = typeText(t, m, longName)
		check("long name", m)
		m = keys(t, m, "enter")
		check("java", m)
		m = keys(t, m, "enter")
		check("summary", m)
	}
}
