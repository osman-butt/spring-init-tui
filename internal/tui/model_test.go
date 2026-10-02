package tui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

type fakeBackend struct {
	metadata initializr.Metadata
	err      error

	generateErr error
	requests    []initializr.Request // what Generate was asked for
	dests       []string
}

func (f *fakeBackend) Metadata(context.Context) (initializr.Metadata, error) {
	return f.metadata, f.err
}

func (f *fakeBackend) Generate(_ context.Context, req initializr.Request, dest string) error {
	f.requests = append(f.requests, req)
	f.dests = append(f.dests, dest)
	return f.generateErr
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
	case "left":
		return tea.KeyPressMsg{Code: tea.KeyLeft}
	case "right":
		return tea.KeyPressMsg{Code: tea.KeyRight}
	case "tab":
		return tea.KeyPressMsg{Code: tea.KeyTab}
	case "space":
		return tea.KeyPressMsg{Code: tea.KeySpace, Text: " "}
	}
	return tea.KeyPressMsg{Code: []rune(s)[0], Text: s}
}

var ansiSequence = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// plain returns the rendered view without colours and text styles.
func plain(m Model) string {
	return ansiSequence.ReplaceAllString(m.View().Content, "")
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

// settle runs cmd and feeds what it produces back into the model, the way
// the Bubble Tea runtime would. Commands that take a while, such as cursor
// blink timers, are skipped.
func settle(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return m
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, c := range batch {
				m = settle(t, m, c)
			}
			return m
		}
		next, nextCmd := step(t, m, msg)
		return settle(t, next, nextCmd)
	case <-time.After(50 * time.Millisecond):
		return m
	}
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
	m := New(&fakeBackend{metadata: testMetadata})
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
	msg := New(&fakeBackend{metadata: testMetadata}).loadCmd()()
	got, ok := msg.(metadataLoadedMsg)
	if !ok || len(got.metadata.Dependencies) != 2 {
		t.Errorf("loadCmd() = %#v, want metadataLoadedMsg with 2 dependencies", msg)
	}

	msg = New(&fakeBackend{err: errors.New("boom")}).loadCmd()()
	if failed, ok := msg.(loadFailedMsg); !ok || failed.err.Error() != "boom" {
		t.Errorf("loadCmd() = %#v, want loadFailedMsg{boom}", msg)
	}
}

func TestLoadingShowsSpinnerText(t *testing.T) {
	m := New(&fakeBackend{})
	if m.screen != screenLoading {
		t.Fatalf("screen = %v, want loading", m.screen)
	}
	out := plain(m)
	for _, want := range []string{"SPRING INITIALIZR", "Build. Configure. Generate.", "Fetching metadata"} {
		if !strings.Contains(out, want) {
			t.Errorf("view is missing %q:\n%s", want, out)
		}
	}
}

func TestMetadataLoadedShowsNameScreen(t *testing.T) {
	m := New(&fakeBackend{})
	m, cmd := step(t, m, metadataLoadedMsg{testMetadata})
	if m.screen != screenName {
		t.Fatalf("screen = %v, want name", m.screen)
	}
	if !m.input.Focused() || cmd == nil {
		t.Error("the name input should be focused and return its blink command")
	}
	if !strings.Contains(plain(m), "Project") {
		t.Errorf("name view is missing its header:\n%s", plain(m))
	}
}

func TestLoadFailureCanBeRetried(t *testing.T) {
	m := New(&fakeBackend{metadata: testMetadata})
	m, _ = step(t, m, loadFailedMsg{errors.New("boom")})
	if m.screen != screenError {
		t.Fatalf("screen = %v, want error", m.screen)
	}
	out := plain(m)
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
	if strings.Contains(plain(m), "retry") {
		t.Error("retry key should not be offered while loading")
	}
}

func TestRetryIsIgnoredWhileLoading(t *testing.T) {
	m := New(&fakeBackend{})
	m, _ = step(t, m, press("r"))
	if m.screen != screenLoading {
		t.Errorf("screen = %v, want loading", m.screen)
	}
}

func TestQuit(t *testing.T) {
	m := New(&fakeBackend{})
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
	m := New(&fakeBackend{})
	m, _ = step(t, m, loadFailedMsg{errors.New("boom")})
	m, _ = step(t, m, press("q"))

	if m.Err() == nil {
		t.Error("Err() = nil, want the load error")
	}
	out := plain(m)
	if !strings.Contains(out, "boom") {
		t.Errorf("final view should keep the error:\n%s", out)
	}
	if strings.Contains(out, "retry") {
		t.Errorf("final view should not show the help footer:\n%s", out)
	}
}

func TestCtrlCInterrupts(t *testing.T) {
	m := New(&fakeBackend{})
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
			if out := plain(m); !strings.Contains(out, tt.wantErr) {
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
	if out := plain(m); strings.Contains(out, "q quit") {
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
	if out := plain(m); !strings.Contains(out, "> 21") {
		t.Errorf("java view should mark 21 as selected:\n%s", out)
	}

	m = keys(t, m, "enter")
	if m.screen != screenDeps {
		t.Fatalf("screen = %v, want deps", m.screen)
	}
}

func TestJavaDefaultFallsBackToFirstVersion(t *testing.T) {
	m := New(&fakeBackend{})
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
	m = keys(t, m, "enter", "up", "enter", "space", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want summary", m.screen)
	}

	m = keys(t, m, "esc")
	if m.screen != screenDeps || !slices.Equal(m.selectedDeps(), []string{"web"}) {
		t.Fatalf("esc on summary: screen = %v, deps = %v; want deps screen with web", m.screen, m.selectedDeps())
	}

	m = keys(t, m, "esc")
	if m.screen != screenJava || m.javaVersion() != "21" {
		t.Fatalf("esc on deps: screen = %v, java = %q; want java screen with 21", m.screen, m.javaVersion())
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
	for _, screen := range []screen{screenJava, screenDeps, screenConfirm} {
		m = keys(t, m, "enter")
		if m.screen != screen {
			t.Fatalf("screen = %v, want %v", m.screen, screen)
		}
		_, cmd := step(t, m, press("q"))
		assertQuits(t, cmd)
	}
}

// onDeps returns a model on the dependency screen.
func onDeps(t *testing.T) Model {
	t.Helper()
	m := typeText(t, loaded(t), "demo")
	m = keys(t, m, "enter", "enter")
	if m.screen != screenDeps {
		t.Fatalf("screen = %v, want deps", m.screen)
	}
	return m
}

func TestDepsToggle(t *testing.T) {
	m := onDeps(t)
	if got := m.selectedDeps(); len(got) != 0 {
		t.Fatalf("selectedDeps() = %v, want none", got)
	}
	if out := plain(m); !strings.Contains(out, "[ ] Spring Web") || !strings.Contains(out, "Selected: none") {
		t.Errorf("deps view should list unchecked items:\n%s", out)
	}

	m = keys(t, m, "space")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web"}) {
		t.Errorf("after space: selectedDeps() = %v, want [web]", got)
	}
	if out := plain(m); !strings.Contains(out, "[x] Spring Web") || !strings.Contains(out, "Selected (1): web") {
		t.Errorf("deps view should show web as checked:\n%s", out)
	}

	m = keys(t, m, "down", "tab")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web", "data-jpa"}) {
		t.Errorf("after down, tab: selectedDeps() = %v, want [web data-jpa]", got)
	}

	m = keys(t, m, "space")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web"}) {
		t.Errorf("after toggling data-jpa off: selectedDeps() = %v, want [web]", got)
	}
}

func TestDepsKeepMetadataOrder(t *testing.T) {
	m := keys(t, onDeps(t), "down", "space", "up", "space")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web", "data-jpa"}) {
		t.Errorf("selectedDeps() = %v, want metadata order [web data-jpa]", got)
	}
}

func TestDepsToggleDoesNotChangeEarlierModels(t *testing.T) {
	before := onDeps(t)
	after := keys(t, before, "space")
	if len(before.selectedDeps()) != 0 || len(after.selectedDeps()) != 1 {
		t.Errorf("before = %v, after = %v; want the toggle to affect only the new model",
			before.selectedDeps(), after.selectedDeps())
	}
}

func TestDepsFilterTakesOverTheKeyboard(t *testing.T) {
	m := keys(t, onDeps(t), "/")
	if m.deps.FilterState() != list.Filtering {
		t.Fatalf("filter state = %v, want filtering", m.deps.FilterState())
	}

	// q and space are filter text now, not shortcuts.
	m, cmd := step(t, m, press("q"))
	if cmd != nil {
		if _, quit := cmd().(tea.QuitMsg); quit {
			t.Fatal("q quit the program while typing a filter")
		}
	}
	m = keys(t, m, "space")
	if m.screen != screenDeps || len(m.selectedDeps()) != 0 {
		t.Errorf("screen = %v, deps = %v; typing a filter should not select anything", m.screen, m.selectedDeps())
	}
	if got := m.deps.FilterInput.Value(); got != "q " {
		t.Errorf("filter text = %q, want %q", got, "q ")
	}

	out := plain(m)
	if !strings.Contains(out, "apply filter") || strings.Contains(out, "q quit") {
		t.Errorf("footer should show the filter keys only:\n%s", out)
	}

	m = keys(t, m, "esc")
	if m.screen != screenDeps || m.deps.FilterState() != list.Unfiltered {
		t.Errorf("esc while filtering: screen = %v, filter = %v; want deps, unfiltered", m.screen, m.deps.FilterState())
	}
}

func TestDepsFilterByTyping(t *testing.T) {
	m := New(&fakeBackend{})
	m.exists = func(string) bool { return false }
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, metadataLoadedMsg{manyDeps()})
	m = keys(t, typeText(t, m, "demo"), "enter", "enter")
	if m.deps.Paginator.TotalPages < 2 {
		t.Fatalf("test needs several pages, got %d", m.deps.Paginator.TotalPages)
	}

	m = keys(t, m, "/")
	for _, r := range "name 07" {
		var cmd tea.Cmd
		m, cmd = step(t, m, press(string(r)))
		m = settle(t, m, cmd)
	}

	out := plain(m)
	if !strings.Contains(out, "Display Name 07") || strings.Contains(out, "Display Name 08") {
		t.Errorf("only dependency 07 should be listed:\n%s", out)
	}
	if m.deps.Paginator.TotalPages != 1 {
		t.Errorf("TotalPages = %d after filtering down to one match, want 1", m.deps.Paginator.TotalPages)
	}

	m = keys(t, m, "enter", "space")
	if m.deps.FilterState() != list.FilterApplied {
		t.Fatalf("filter state = %v, want applied", m.deps.FilterState())
	}
	if got := m.selectedDeps(); !slices.Equal(got, []string{"dependency-with-a-long-identifier-07"}) {
		t.Errorf("selectedDeps() = %v, want dependency 07", got)
	}
}

func TestDepsAppliedFilter(t *testing.T) {
	m := onDeps(t)
	m.deps.SetFilterText("jpa")
	m.syncKeys()
	if m.deps.FilterState() != list.FilterApplied {
		t.Fatalf("filter state = %v, want applied", m.deps.FilterState())
	}
	out := plain(m)
	if strings.Contains(out, "Spring Web") || !strings.Contains(out, "Spring Data JPA") {
		t.Errorf("only the JPA dependency should be listed:\n%s", out)
	}
	if !strings.Contains(out, "clear filter") {
		t.Errorf("footer should offer to clear the filter:\n%s", out)
	}

	// The cursor is on the first match, not on the first dependency.
	m = keys(t, m, "space")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"data-jpa"}) {
		t.Errorf("selectedDeps() = %v, want [data-jpa]", got)
	}

	// esc clears the filter first, and only then goes back.
	m = keys(t, m, "esc")
	if m.screen != screenDeps || m.deps.FilterState() != list.Unfiltered {
		t.Fatalf("first esc: screen = %v, filter = %v; want deps, unfiltered", m.screen, m.deps.FilterState())
	}
	if got := m.selectedDeps(); !slices.Equal(got, []string{"data-jpa"}) {
		t.Errorf("clearing the filter changed the selection: %v", got)
	}
	m = keys(t, m, "esc")
	if m.screen != screenJava {
		t.Errorf("second esc: screen = %v, want java", m.screen)
	}
}

func TestDepsFilterWithoutMatches(t *testing.T) {
	m := onDeps(t)
	m.deps.SetFilterText("no such dependency")
	m = keys(t, m, "space")
	if got := m.selectedDeps(); len(got) != 0 {
		t.Errorf("selectedDeps() = %v, want none", got)
	}
}

func TestSummaryShowsAnswers(t *testing.T) {
	m := keys(t, onDeps(t), "enter")
	if out := plain(m); !strings.Contains(out, "none") {
		t.Errorf("summary without dependencies should say none:\n%s", out)
	}

	m = keys(t, onDeps(t), "space", "down", "space", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want summary", m.screen)
	}
	out := plain(m)
	for _, want := range []string{"demo", "17", "web, data-jpa"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}
}

// onConfirm returns a model on the confirm prompt for project "demo", Java
// 17 and the web dependency, together with the backend it talks to.
func onConfirm(t *testing.T) (Model, *fakeBackend) {
	t.Helper()
	backend := &fakeBackend{metadata: testMetadata}
	m := New(backend)
	m.exists = func(string) bool { return false }
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, metadataLoadedMsg{testMetadata})
	m = keys(t, typeText(t, m, "demo"), "enter", "enter", "space", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want confirm", m.screen)
	}
	return m, backend
}

func TestConfirmYesGeneratesTheProject(t *testing.T) {
	m, backend := onConfirm(t)
	if out := plain(m); !strings.Contains(out, "Generate demo?") {
		t.Errorf("confirm view should ask the question:\n%s", out)
	}

	m, cmd := step(t, m, press("enter")) // Yes is preselected
	if m.screen != screenGenerating || cmd == nil {
		t.Fatalf("screen = %v, cmd = %v; want generating with a command", m.screen, cmd)
	}
	if out := plain(m); !strings.Contains(out, "Generating demo") {
		t.Errorf("generating view should name the project:\n%s", out)
	}

	msg := m.generateCmd()()
	if _, ok := msg.(generatedMsg); !ok {
		t.Fatalf("generateCmd() = %#v, want generatedMsg", msg)
	}
	want := initializr.Request{Name: "demo", JavaVersion: "17", Dependencies: []string{"web"}}
	if len(backend.requests) != 1 || !reflect.DeepEqual(backend.requests[0], want) {
		t.Errorf("Generate requests = %+v, want [%+v]", backend.requests, want)
	}
	if !slices.Equal(backend.dests, []string{"demo"}) {
		t.Errorf("Generate dests = %v, want [demo]", backend.dests)
	}

	m, cmd = step(t, m, msg)
	assertQuits(t, cmd)
	if m.Err() != nil || m.Interrupted() {
		t.Errorf("Err() = %v, Interrupted() = %v; want a clean exit", m.Err(), m.Interrupted())
	}
	out := plain(m)
	for _, want := range []string{"Project generated successfully.", "cd demo", "./mvnw spring-boot:run"} {
		if !strings.Contains(out, want) {
			t.Errorf("result is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "quit") {
		t.Errorf("result should not show the help footer:\n%s", out)
	}
}

func TestConfirmAnswers(t *testing.T) {
	tests := []struct {
		name     string
		keys     []string
		generate bool
	}{
		{name: "enter on the default", keys: []string{"enter"}, generate: true},
		{name: "y", keys: []string{"y"}, generate: true},
		{name: "y while No is highlighted", keys: []string{"right", "y"}, generate: true},
		{name: "switch twice, enter", keys: []string{"right", "left", "enter"}, generate: true},
		{name: "switch to No, enter", keys: []string{"right", "enter"}},
		{name: "tab to No, enter", keys: []string{"tab", "enter"}},
		{name: "n", keys: []string{"n"}},
		{name: "q", keys: []string{"q"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := onConfirm(t)
			var cmd tea.Cmd
			for _, k := range tt.keys {
				m, cmd = step(t, m, press(k))
			}

			if tt.generate {
				if m.screen != screenGenerating || cmd == nil {
					t.Errorf("screen = %v, cmd = %v; want generating with a command", m.screen, cmd)
				}
				return
			}
			assertQuits(t, cmd)
			if m.Err() != nil || m.Interrupted() {
				t.Errorf("Err() = %v, Interrupted() = %v; declining should exit cleanly", m.Err(), m.Interrupted())
			}
			if out := m.View().Content; out != m.bannerView()+"\n" {
				t.Errorf("view after declining should be the banner only, got:\n%s", out)
			}
		})
	}
}

func TestConfirmBackKeepsAnswers(t *testing.T) {
	m, _ := onConfirm(t)
	m = keys(t, m, "esc")
	if m.screen != screenDeps || !slices.Equal(m.selectedDeps(), []string{"web"}) {
		t.Errorf("screen = %v, deps = %v; want deps screen with web", m.screen, m.selectedDeps())
	}
}

func TestGenerateFailure(t *testing.T) {
	backend := &fakeBackend{metadata: testMetadata, generateErr: errors.New("demo already exists")}
	m, _ := onConfirm(t)
	m.backend = backend
	m = keys(t, m, "enter")

	msg := m.generateCmd()()
	if failed, ok := msg.(generateFailedMsg); !ok || failed.err.Error() != "demo already exists" {
		t.Fatalf("generateCmd() = %#v, want generateFailedMsg", msg)
	}

	m, _ = step(t, m, msg)
	if m.screen != screenError {
		t.Fatalf("screen = %v, want error", m.screen)
	}
	out := plain(m)
	for _, want := range []string{"demo already exists", "retry", "back"} {
		if !strings.Contains(out, want) {
			t.Errorf("error view is missing %q:\n%s", want, out)
		}
	}

	// r tries the generation again, not the metadata download.
	retried, cmd := step(t, m, press("r"))
	if retried.screen != screenGenerating || retried.err != nil || cmd == nil {
		t.Errorf("after r: screen = %v, err = %v, cmd = %v; want generating", retried.screen, retried.err, cmd)
	}

	back := keys(t, m, "esc")
	if back.screen != screenConfirm || back.err != nil || back.name != "demo" {
		t.Errorf("after esc: screen = %v, err = %v, name = %q; want confirm with answers", back.screen, back.err, back.name)
	}

	quit, cmd := step(t, m, press("q"))
	assertQuits(t, cmd)
	if quit.Err() == nil || !strings.Contains(plain(quit), "demo already exists") {
		t.Errorf("quitting from the error should keep it: Err() = %v", quit.Err())
	}
}

func TestLoadFailureOffersNoWayBack(t *testing.T) {
	m := New(&fakeBackend{})
	m, _ = step(t, m, loadFailedMsg{errors.New("boom")})
	if out := plain(m); strings.Contains(out, "back") {
		t.Errorf("there is nothing to go back to after a load failure:\n%s", out)
	}
	if m = keys(t, m, "esc"); m.screen != screenError {
		t.Errorf("esc on a load failure: screen = %v, want error", m.screen)
	}
}

func TestQuitWhileGeneratingIgnoresLateResults(t *testing.T) {
	for name, late := range map[string]tea.Msg{
		"cancelled request": generateFailedMsg{context.Canceled},
		"finished anyway":   generatedMsg{},
	} {
		t.Run(name, func(t *testing.T) {
			m, _ := onConfirm(t)
			m = keys(t, m, "enter")
			m, cmd := step(t, m, press("q"))
			assertQuits(t, cmd)
			if m.ctx.Err() == nil {
				t.Error("quitting should cancel the request")
			}

			m, _ = step(t, m, late)
			if m.Err() != nil {
				t.Errorf("Err() = %v, want nil", m.Err())
			}
			if out := m.View().Content; out != m.bannerView()+"\n" {
				t.Errorf("view should stay the banner only, got:\n%s", out)
			}
		})
	}
}

// manyDeps returns metadata with enough dependencies to need several pages.
func manyDeps() initializr.Metadata {
	md := testMetadata
	md.Dependencies = nil
	for i := range 60 {
		md.Dependencies = append(md.Dependencies, initializr.Dependency{
			ID:   fmt.Sprintf("dependency-with-a-long-identifier-%02d", i),
			Name: fmt.Sprintf("Dependency With A Rather Long Display Name %02d", i),
		})
	}
	return md
}

func TestViewFitsWindow(t *testing.T) {
	longErr := errors.New(strings.Repeat("something went wrong ", 10))
	longName := strings.Repeat("a", 64)

	for _, size := range [][2]int{{120, 40}, {80, 24}, {40, 16}, {24, 12}} {
		width, height := size[0], size[1]
		check := func(screen string, m Model) {
			t.Helper()
			w, h := lipgloss.Size(m.View().Content)
			if w > width {
				t.Errorf("%s view is %d wide in a %dx%d window", screen, w, width, height)
			}
			// Wrapped text may legitimately need more rows than a tiny
			// window has. The other screens must always fit.
			if h > height && !strings.Contains(screen, "error") && !strings.Contains(screen, "summary") && screen != "done" {
				t.Errorf("%s view is %d high in a %dx%d window", screen, h, width, height)
			}
		}

		m := New(&fakeBackend{})
		m.exists = func(string) bool { return false }
		m, _ = step(t, m, tea.WindowSizeMsg{Width: width, Height: height})
		check("loading", m)

		failed, _ := step(t, m, loadFailedMsg{longErr})
		check("error", failed)

		m, _ = step(t, m, metadataLoadedMsg{manyDeps()})
		check("empty name", m)
		check("invalid name", keys(t, typeText(t, m, "not valid!"), "enter"))

		m = typeText(t, m, longName)
		check("long name", m)
		m = keys(t, m, "enter")
		check("java", m)
		m = keys(t, m, "enter")
		check("deps", m)
		check("deps filter", keys(t, m, "/", "x"))

		// Select a few pages worth of dependencies.
		for range 3 {
			m = keys(t, m, "space", "down", "space", "down", "space", "right")
		}
		check("deps with selection", m)
		m = keys(t, m, "enter")
		check("summary", m)
		check("summary, No highlighted", keys(t, m, "right"))
		m = keys(t, m, "enter")
		check("generating", m)
		done, _ := step(t, m, generatedMsg{})
		check("done", done)
		failed, _ = step(t, m, generateFailedMsg{longErr})
		check("generate error", failed)
	}
}

func TestDepsListHeightStaysTheSame(t *testing.T) {
	m := New(&fakeBackend{})
	m.exists = func(string) bool { return false }
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, metadataLoadedMsg{manyDeps()})
	m = keys(t, typeText(t, m, "demo"), "enter", "enter")

	want := lipgloss.Height(m.View().Content)
	last := keys(t, m, "G")
	m.deps.SetFilterText("05")
	for name, variant := range map[string]Model{"last page": last, "one match": m, "typing filter": keys(t, m, "/")} {
		if got := lipgloss.Height(variant.View().Content); got != want {
			t.Errorf("%s: frame is %d lines, want %d", name, got, want)
		}
	}
}
