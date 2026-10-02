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
	if s == "ctrl+c" {
		return tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
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

func TestMetadataLoaded(t *testing.T) {
	m := New(fakeBackend{})
	m, _ = step(t, m, metadataLoadedMsg{testMetadata})
	if m.screen != screenReady {
		t.Fatalf("screen = %v, want ready", m.screen)
	}
	if m.metadata.DefaultJavaVersion != "17" {
		t.Errorf("metadata was not stored: %+v", m.metadata)
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
	if cmd == nil {
		t.Fatal("q returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("q should quit")
	}
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
	if cmd == nil {
		t.Fatal("ctrl+c returned no command")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Error("ctrl+c should quit")
	}
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

func TestViewFitsWidth(t *testing.T) {
	longErr := errors.New(strings.Repeat("something went wrong ", 10))
	for _, width := range []int{120, 80, 40, 24} {
		m := New(fakeBackend{})
		m, _ = step(t, m, tea.WindowSizeMsg{Width: width, Height: 24})
		if w := lipgloss.Width(m.View().Content); w > width {
			t.Errorf("loading view is %d wide at width %d", w, width)
		}

		m, _ = step(t, m, loadFailedMsg{longErr})
		if w := lipgloss.Width(m.View().Content); w > width {
			t.Errorf("error view is %d wide at width %d", w, width)
		}
	}
}
