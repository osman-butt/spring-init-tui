package tui

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"testing"

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
	DefaultGroupID:     "com.example",
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
	case "backspace":
		return tea.KeyPressMsg{Code: tea.KeyBackspace}
	case "pgup":
		return tea.KeyPressMsg{Code: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyPressMsg{Code: tea.KeyPgDown}
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
	for _, want := range []string{"I N I T I A L I Z R", "Fetching metadata"} {
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
	if !strings.Contains(plain(m), "Artifact") {
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
		{name: "empty", input: "", wantErr: "enter an artifact name"},
		{name: "only spaces", input: "   ", wantErr: "enter an artifact name"},
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
	if m.screen != screenGroup || m.name != "my-app_1.0" {
		t.Errorf("screen = %v, name = %q; want group screen with %q", m.screen, m.name, "my-app_1.0")
	}
}

// onGroup returns a model on the group screen for project "demo".
func onGroup(t *testing.T) Model {
	t.Helper()
	m, cmd := step(t, typeText(t, loaded(t), "demo"), press("enter"))
	if m.screen != screenGroup {
		t.Fatalf("screen = %v, want group", m.screen)
	}
	if !m.group.Focused() || m.input.Focused() || cmd == nil {
		t.Fatal("the group input should have taken the focus from the name input")
	}
	return m
}

// replaceGroup clears the prefilled group and types a new one.
func replaceGroup(t *testing.T, m Model, group string) Model {
	t.Helper()
	for range m.group.Value() {
		m = keys(t, m, "backspace")
	}
	return typeText(t, m, group)
}

func TestGroupIsPrefilledFromMetadata(t *testing.T) {
	m := onGroup(t)
	if got := m.group.Value(); got != "com.example" {
		t.Errorf("group input = %q, want the metadata default com.example", got)
	}
	out := plain(m)
	if !strings.Contains(out, "Group") || !strings.Contains(out, "com.example") {
		t.Errorf("group view should show its header and the default:\n%s", out)
	}

	m = keys(t, m, "enter")
	if m.screen != screenPackage || m.groupID != "com.example" {
		t.Errorf("screen = %v, groupID = %q; want package screen with com.example", m.screen, m.groupID)
	}
}

func TestGroupFallsBackWithoutMetadataDefault(t *testing.T) {
	m := New(&fakeBackend{})
	m, _ = step(t, m, metadataLoadedMsg{initializr.Metadata{JavaVersions: []string{"21"}}})
	if got := m.group.Value(); got != "com.example" {
		t.Errorf("group input = %q, want the fallback com.example", got)
	}
}

func TestGroupValidation(t *testing.T) {
	tests := []struct {
		name, input, wantErr string
	}{
		{name: "empty", input: "", wantErr: "enter a group"},
		{name: "hyphen", input: "com.my-company", wantErr: "use dot-separated names"},
		{name: "empty segment", input: "com..example", wantErr: "use dot-separated names"},
		{name: "trailing dot", input: "com.example.", wantErr: "use dot-separated names"},
		{name: "segment starts with a digit", input: "com.1example", wantErr: "use dot-separated names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := keys(t, replaceGroup(t, onGroup(t), tt.input), "enter")
			if m.screen != screenGroup {
				t.Fatalf("screen = %v, want to stay on group", m.screen)
			}
			if out := plain(m); !strings.Contains(out, tt.wantErr) {
				t.Errorf("view should explain the problem (%q):\n%s", tt.wantErr, out)
			}

			m = typeText(t, m, "x")
			if m.groupErr != nil {
				t.Errorf("groupErr = %v, want it cleared after typing", m.groupErr)
			}
		})
	}
}

func TestGroupAcceptsLettersThatAreShortcutsElsewhere(t *testing.T) {
	m := typeText(t, replaceGroup(t, onGroup(t), ""), "qjk.r")
	if m.screen != screenGroup || m.group.Value() != "qjk.r" {
		t.Errorf("screen = %v, group = %q; want group screen with %q", m.screen, m.group.Value(), "qjk.r")
	}
	out := plain(m)
	if strings.Contains(out, "q quit") || !strings.Contains(out, "ctrl+c quit") {
		t.Errorf("footer should offer ctrl+c, not q, while typing:\n%s", out)
	}
}

func TestPackageIsDerivedFromGroupAndName(t *testing.T) {
	backend := &fakeBackend{metadata: testMetadata}
	m := New(backend)
	m.exists = func(string) bool { return false }
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, metadataLoadedMsg{testMetadata})
	m = keys(t, typeText(t, m, "spring-demo"), "enter")
	m = keys(t, replaceGroup(t, m, " dev.osman "), "enter", "enter", "enter", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want confirm", m.screen)
	}

	out := plain(m)
	for _, want := range []string{"spring-demo", "dev.osman", "dev.osman.springdemo"} {
		if !strings.Contains(out, want) {
			t.Errorf("summary is missing %q:\n%s", want, out)
		}
	}

	m.generateCmd()()
	want := initializr.Request{
		Name:        "spring-demo",
		GroupID:     "dev.osman",
		PackageName: "dev.osman.springdemo",
		JavaVersion: "17",
	}
	if len(backend.requests) != 1 || !reflect.DeepEqual(backend.requests[0], want) {
		t.Errorf("Generate requests = %+v, want [%+v]", backend.requests, want)
	}
	if !slices.Equal(backend.dests, []string{"spring-demo"}) {
		t.Errorf("Generate dests = %v, want [spring-demo]", backend.dests)
	}
}

// onPackage returns a model on the package screen for project "demo" in the
// default group.
func onPackage(t *testing.T) Model {
	t.Helper()
	m, cmd := step(t, onGroup(t), press("enter"))
	if m.screen != screenPackage {
		t.Fatalf("screen = %v, want package", m.screen)
	}
	if !m.pkg.Focused() || m.group.Focused() || cmd == nil {
		t.Fatal("the package input should have taken the focus from the group input")
	}
	return m
}

// replacePackage clears the prefilled package and types a new one.
func replacePackage(t *testing.T, m Model, name string) Model {
	t.Helper()
	for range m.pkg.Value() {
		m = keys(t, m, "backspace")
	}
	return typeText(t, m, name)
}

func TestPackageIsPrefilledFromGroupAndName(t *testing.T) {
	m := onPackage(t)
	if got := m.pkg.Value(); got != "com.example.demo" {
		t.Errorf("package input = %q, want com.example.demo", got)
	}
	out := plain(m)
	if !strings.Contains(out, "Package name") || !strings.Contains(out, "com.example.demo") {
		t.Errorf("package view should show its header and the default:\n%s", out)
	}

	m = keys(t, m, "enter")
	if m.screen != screenJava || m.packageName != "com.example.demo" {
		t.Errorf("screen = %v, packageName = %q; want java screen with com.example.demo", m.screen, m.packageName)
	}
}

func TestUntouchedPackageFollowsTheGroup(t *testing.T) {
	m := keys(t, onPackage(t), "esc")
	m = keys(t, replaceGroup(t, m, "dev.osman"), "enter")
	if m.screen != screenPackage || m.pkg.Value() != "dev.osman.demo" {
		t.Errorf("screen = %v, package = %q; want package screen with dev.osman.demo", m.screen, m.pkg.Value())
	}
	if got := m.pkg.Position(); got != len("dev.osman.demo") {
		t.Errorf("cursor at %d, want it at the end", got)
	}
}

func TestEditedPackageIsKept(t *testing.T) {
	backend := &fakeBackend{metadata: testMetadata}
	m := replacePackage(t, onPackage(t), " dev.custom ")
	m.backend = backend

	// Changing the group afterwards must not overwrite it.
	m = keys(t, m, "esc")
	m = keys(t, replaceGroup(t, m, "dev.osman"), "enter")
	if got := m.pkg.Value(); got != " dev.custom " {
		t.Fatalf("package input = %q, want the typed package to be kept", got)
	}

	m = keys(t, m, "enter", "enter", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want confirm", m.screen)
	}
	if out := plain(m); !strings.Contains(out, "dev.custom") || strings.Contains(out, "dev.osman.demo") {
		t.Errorf("summary should show the typed package:\n%s", out)
	}

	m.generateCmd()()
	if len(backend.requests) != 1 || backend.requests[0].PackageName != "dev.custom" || backend.requests[0].GroupID != "dev.osman" {
		t.Errorf("Generate requests = %+v, want group dev.osman with package dev.custom", backend.requests)
	}
}

func TestPackageValidation(t *testing.T) {
	tests := []struct {
		name, input, wantErr string
	}{
		{name: "empty", input: "", wantErr: "enter a package name"},
		{name: "hyphen", input: "com.example.my-app", wantErr: "use dot-separated names"},
		{name: "empty segment", input: "com..demo", wantErr: "use dot-separated names"},
		{name: "trailing dot", input: "com.example.", wantErr: "use dot-separated names"},
		{name: "segment starts with a digit", input: "com.example.2048", wantErr: "use dot-separated names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := keys(t, replacePackage(t, onPackage(t), tt.input), "enter")
			if m.screen != screenPackage {
				t.Fatalf("screen = %v, want to stay on package", m.screen)
			}
			if out := plain(m); !strings.Contains(out, tt.wantErr) {
				t.Errorf("view should explain the problem (%q):\n%s", tt.wantErr, out)
			}

			m = typeText(t, m, "x")
			if m.packageErr != nil {
				t.Errorf("packageErr = %v, want it cleared after typing", m.packageErr)
			}
		})
	}
}

func TestPackageAcceptsLettersThatAreShortcutsElsewhere(t *testing.T) {
	m := typeText(t, replacePackage(t, onPackage(t), ""), "qjk.r")
	if m.screen != screenPackage || m.pkg.Value() != "qjk.r" {
		t.Errorf("screen = %v, package = %q; want package screen with %q", m.screen, m.pkg.Value(), "qjk.r")
	}
	out := plain(m)
	if strings.Contains(out, "q quit") || !strings.Contains(out, "ctrl+c quit") {
		t.Errorf("footer should offer ctrl+c, not q, while typing:\n%s", out)
	}
}

func TestJavaVersionSelection(t *testing.T) {
	m := keys(t, onPackage(t), "enter")
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
	m := keys(t, replaceGroup(t, onGroup(t), "dev.osman"), "enter", "enter", "up", "enter", "tab", "enter")
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
	if m.screen != screenPackage || m.pkg.Value() != "dev.osman.demo" {
		t.Fatalf("esc on java: screen = %v, package = %q; want package screen with dev.osman.demo", m.screen, m.pkg.Value())
	}
	if !m.pkg.Focused() || cmd == nil {
		t.Error("the package input should be focused again")
	}

	m, cmd = step(t, m, press("esc"))
	if m.screen != screenGroup || m.group.Value() != "dev.osman" {
		t.Fatalf("esc on package: screen = %v, group = %q; want group screen with dev.osman", m.screen, m.group.Value())
	}
	if !m.group.Focused() || m.pkg.Focused() || cmd == nil {
		t.Error("the group input should be focused again")
	}

	m, cmd = step(t, m, press("esc"))
	if m.screen != screenName || m.input.Value() != "demo" {
		t.Fatalf("esc on group: screen = %v, input = %q; want name screen with demo", m.screen, m.input.Value())
	}
	if !m.input.Focused() || m.group.Focused() || cmd == nil {
		t.Error("the name input should be focused again")
	}

	_, cmd = step(t, m, press("esc"))
	assertQuits(t, cmd)
}

func TestQuitFromFormScreens(t *testing.T) {
	// q is text on the package and dependency screens, so ctrl+c quits there.
	quitKeys := map[screen]string{
		screenPackage: "ctrl+c", screenJava: "q", screenDeps: "ctrl+c", screenConfirm: "q",
	}

	m := onGroup(t)
	for _, screen := range []screen{screenPackage, screenJava, screenDeps, screenConfirm} {
		m = keys(t, m, "enter")
		if m.screen != screen {
			t.Fatalf("screen = %v, want %v", m.screen, screen)
		}
		_, cmd := step(t, m, press(quitKeys[screen]))
		assertQuits(t, cmd)
	}
}

// onDeps returns a model on the dependency screen.
func onDeps(t *testing.T) Model {
	t.Helper()
	m := keys(t, onPackage(t), "enter", "enter")
	if m.screen != screenDeps {
		t.Fatalf("screen = %v, want deps", m.screen)
	}
	return m
}

func TestDepsToggle(t *testing.T) {
	m := onDeps(t)
	if !m.search.Focused() {
		t.Error("the search input should be focused on the dependency screen")
	}
	if got := m.selectedDeps(); len(got) != 0 {
		t.Fatalf("selectedDeps() = %v, want none", got)
	}
	out := plain(m)
	for _, want := range []string{"Type to search", "2 dependencies", "[ ] Spring Web", "Selected: none"} {
		if !strings.Contains(out, want) {
			t.Errorf("deps view is missing %q:\n%s", want, out)
		}
	}

	m = keys(t, m, "tab")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web"}) {
		t.Errorf("after tab: selectedDeps() = %v, want [web]", got)
	}
	if out := plain(m); !strings.Contains(out, "[x] Spring Web") || !strings.Contains(out, "Selected (1): web") {
		t.Errorf("deps view should show web as checked:\n%s", out)
	}

	m = keys(t, m, "down", "tab")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web", "data-jpa"}) {
		t.Errorf("after down, tab: selectedDeps() = %v, want [web data-jpa]", got)
	}

	m = keys(t, m, "tab")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web"}) {
		t.Errorf("after toggling data-jpa off: selectedDeps() = %v, want [web]", got)
	}
}

func TestDepsKeepMetadataOrder(t *testing.T) {
	m := keys(t, onDeps(t), "down", "tab", "up", "tab")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web", "data-jpa"}) {
		t.Errorf("selectedDeps() = %v, want metadata order [web data-jpa]", got)
	}
}

func TestDepsToggleDoesNotChangeEarlierModels(t *testing.T) {
	before := onDeps(t)
	after := keys(t, before, "tab")
	if len(before.selectedDeps()) != 0 || len(after.selectedDeps()) != 1 {
		t.Errorf("before = %v, after = %v; want the toggle to affect only the new model",
			before.selectedDeps(), after.selectedDeps())
	}
}

// The flow the search is built for: type, tab, clear, type again, tab.
func TestDepsSearchSelectSearchAgain(t *testing.T) {
	m := typeText(t, onDeps(t), "jpa")
	out := plain(m)
	if strings.Contains(out, "Spring Web") || !strings.Contains(out, "Spring Data JPA") {
		t.Errorf("typing jpa should list only the JPA dependency:\n%s", out)
	}
	if !strings.Contains(out, "1 of 2") {
		t.Errorf("deps view should count the matches:\n%s", out)
	}

	// The cursor is on the best match, so tab selects it straight away.
	m = keys(t, m, "tab")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"data-jpa"}) {
		t.Fatalf("selectedDeps() = %v, want [data-jpa]", got)
	}
	if got := m.search.Value(); got != "jpa" {
		t.Errorf("search = %q, want it to stay after selecting", got)
	}

	m = keys(t, m, "backspace", "backspace", "backspace")
	if out := plain(m); !strings.Contains(out, "Spring Web") || !strings.Contains(out, "2 dependencies") {
		t.Errorf("an empty search should list everything again:\n%s", out)
	}

	m = keys(t, typeText(t, m, "web"), "tab")
	if got := m.selectedDeps(); !slices.Equal(got, []string{"web", "data-jpa"}) {
		t.Errorf("selectedDeps() = %v, want [web data-jpa]", got)
	}
	if m.screen != screenDeps {
		t.Errorf("screen = %v, want deps", m.screen)
	}
}

func TestDepsSearchTakesLettersAndSpace(t *testing.T) {
	m := typeText(t, onDeps(t), "q")
	m = keys(t, m, "space")
	m = typeText(t, m, "jk/")
	if m.screen != screenDeps || len(m.selectedDeps()) != 0 {
		t.Fatalf("screen = %v, deps = %v; typing should neither leave the screen nor select", m.screen, m.selectedDeps())
	}
	if got := m.search.Value(); got != "q jk/" {
		t.Errorf("search = %q, want %q", got, "q jk/")
	}

	out := plain(m)
	if strings.Contains(out, "q quit") || !strings.Contains(out, "ctrl+c quit") {
		t.Errorf("footer should offer ctrl+c, not q, on the dependency screen:\n%s", out)
	}
	if !strings.Contains(out, "No matches.") || !strings.Contains(out, "0 of 2") {
		t.Errorf("deps view should say that nothing matches:\n%s", out)
	}

	// Nothing under the cursor, so tab has nothing to select.
	if m = keys(t, m, "tab"); len(m.selectedDeps()) != 0 {
		t.Errorf("selectedDeps() = %v, want none", m.selectedDeps())
	}
}

func TestDepsEscClearsSearchBeforeGoingBack(t *testing.T) {
	m := keys(t, typeText(t, onDeps(t), "jpa"), "tab")
	if out := plain(m); !strings.Contains(out, "esc clear search") || strings.Contains(out, "esc back") {
		t.Errorf("footer should offer to clear the search:\n%s", out)
	}

	m = keys(t, m, "esc")
	if m.screen != screenDeps || m.search.Value() != "" {
		t.Fatalf("first esc: screen = %v, search = %q; want deps with an empty search", m.screen, m.search.Value())
	}
	if got := m.selectedDeps(); !slices.Equal(got, []string{"data-jpa"}) {
		t.Errorf("clearing the search changed the selection: %v", got)
	}
	out := plain(m)
	if !strings.Contains(out, "Spring Web") || !strings.Contains(out, "esc back") {
		t.Errorf("after clearing, everything is listed and esc goes back:\n%s", out)
	}

	m = keys(t, m, "esc")
	if m.screen != screenJava || m.search.Focused() {
		t.Errorf("second esc: screen = %v, search focused = %v; want java", m.screen, m.search.Focused())
	}
}

func TestDepsSearchSurvivesConfirmAndBack(t *testing.T) {
	m := keys(t, typeText(t, onDeps(t), "jpa"), "tab", "enter")
	if m.screen != screenConfirm || m.search.Focused() {
		t.Fatalf("screen = %v, search focused = %v; want confirm", m.screen, m.search.Focused())
	}

	m, cmd := step(t, m, press("esc"))
	if m.screen != screenDeps || !m.search.Focused() || cmd == nil {
		t.Fatalf("esc on confirm: screen = %v, search focused = %v; want deps with a focused search", m.screen, m.search.Focused())
	}
	if out := plain(m); m.search.Value() != "jpa" || strings.Contains(out, "Spring Web") {
		t.Errorf("the search should still be applied:\n%s", out)
	}
}

func TestDepsArrowsAndPagesMoveWhileSearching(t *testing.T) {
	m := New(&fakeBackend{})
	m.exists = func(string) bool { return false }
	m, _ = step(t, m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m, _ = step(t, m, metadataLoadedMsg{manyDeps()})
	m = keys(t, typeText(t, m, "demo"), "enter", "enter", "enter", "enter")
	if m.deps.Paginator.TotalPages < 2 {
		t.Fatalf("test needs several pages, got %d", m.deps.Paginator.TotalPages)
	}

	m = keys(t, m, "pgdown")
	if m.deps.Paginator.Page != 1 {
		t.Errorf("page after pgdown = %d, want 1", m.deps.Paginator.Page)
	}
	m = keys(t, m, "pgup")
	if m.deps.Paginator.Page != 0 {
		t.Errorf("page after pgup = %d, want 0", m.deps.Paginator.Page)
	}

	// The pages follow the matches as soon as the text changes.
	all := m.deps.Paginator.TotalPages
	m = typeText(t, m, "name 0")
	matches := m.deps.VisibleItems()
	if len(matches) < 3 || len(matches) >= 60 || m.deps.Paginator.TotalPages >= all {
		t.Fatalf("matches = %d, pages = %d (of %d); want a narrowed list", len(matches), m.deps.Paginator.TotalPages, all)
	}

	// The arrows move through the matches without leaving the search.
	m = keys(t, m, "down", "down", "tab")
	third := matches[2].(depItem).ID
	if got := m.selectedDeps(); !slices.Equal(got, []string{third}) {
		t.Errorf("selectedDeps() = %v, want the third match %s", got, third)
	}

	m = typeText(t, m, "7")
	if m.deps.Paginator.TotalPages != 1 || len(m.deps.VisibleItems()) != 1 {
		t.Errorf("pages = %d, matches = %d after narrowing to one; want 1 and 1",
			m.deps.Paginator.TotalPages, len(m.deps.VisibleItems()))
	}
	if out := plain(m); !strings.Contains(out, "Display Name 07") || strings.Contains(out, "Display Name 08") {
		t.Errorf("only dependency 07 should be listed:\n%s", out)
	}
}

func TestSummaryShowsAnswers(t *testing.T) {
	m := keys(t, onDeps(t), "enter")
	if out := plain(m); !strings.Contains(out, "none") {
		t.Errorf("summary without dependencies should say none:\n%s", out)
	}

	m = keys(t, onDeps(t), "tab", "down", "tab", "enter")
	if m.screen != screenConfirm {
		t.Fatalf("screen = %v, want summary", m.screen)
	}
	out := plain(m)
	for _, want := range []string{"demo", "com.example", "com.example.demo", "17", "web, data-jpa"} {
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
	m = keys(t, typeText(t, m, "demo"), "enter", "enter", "enter", "enter", "tab", "enter")
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
	want := initializr.Request{
		Name:         "demo",
		GroupID:      "com.example",
		PackageName:  "com.example.demo",
		JavaVersion:  "17",
		Dependencies: []string{"web"},
	}
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
	for _, want := range []string{"Project generated successfully.", "cd demo", runCommand(runtime.GOOS)} {
		if !strings.Contains(out, want) {
			t.Errorf("result is missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "quit") {
		t.Errorf("result should not show the help footer:\n%s", out)
	}
}

func TestRunCommand(t *testing.T) {
	tests := map[string]string{
		"linux":   "./mvnw spring-boot:run",
		"darwin":  "./mvnw spring-boot:run",
		"windows": `.\mvnw.cmd spring-boot:run`,
	}
	for goos, want := range tests {
		if got := runCommand(goos); got != want {
			t.Errorf("runCommand(%q) = %q, want %q", goos, got, want)
		}
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
		check("group", m)
		check("invalid group", keys(t, typeText(t, m, "-"), "enter"))
		m = typeText(t, m, strings.Repeat(".long", 15))
		check("long group", m)
		m = keys(t, m, "enter")
		check("long package", m)
		check("invalid package", keys(t, typeText(t, m, "-"), "enter"))
		m = keys(t, m, "enter")
		check("java", m)
		m = keys(t, m, "enter")
		check("deps", m)
		check("deps search", typeText(t, m, "name 1"))
		check("deps search without matches", typeText(t, m, "zzz"))

		// Select a few pages worth of dependencies.
		for range 3 {
			m = keys(t, m, "tab", "down", "tab", "down", "tab", "pgdown")
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
	m = keys(t, typeText(t, m, "demo"), "enter", "enter", "enter", "enter")

	want := lipgloss.Height(m.View().Content)
	variants := map[string]Model{
		"next page":  keys(t, m, "pgdown"),
		"one match":  typeText(t, m, "name 05"),
		"no matches": typeText(t, m, "zzz"),
	}
	for name, variant := range variants {
		if got := lipgloss.Height(variant.View().Content); got != want {
			t.Errorf("%s: frame is %d lines, want %d", name, got, want)
		}
	}
}
