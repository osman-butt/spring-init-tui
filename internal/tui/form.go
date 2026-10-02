package tui

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

// The name becomes both the Maven artifactId and the project directory.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func newNameInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = "demo"
	input.CharLimit = 64
	return input
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func (m Model) validateName(name string) error {
	switch {
	case name == "":
		return errors.New("enter a project name")
	case !namePattern.MatchString(name):
		return errors.New("use letters, digits, '.', '-' and '_' only")
	case m.exists(name):
		return fmt.Errorf("%q already exists in this directory", name)
	}
	return nil
}

func (m Model) updateName(msg tea.Msg) (tea.Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, m.keys.Cancel):
			return m.quit()
		case key.Matches(k, m.keys.Next):
			name := strings.TrimSpace(m.input.Value())
			if m.nameErr = m.validateName(name); m.nameErr != nil {
				return m, nil
			}
			m.name = name
			m.input.Blur()
			m.setScreen(screenJava)
			return m, nil
		}
		m.nameErr = nil
	}

	// Everything else, including "q", belongs to the text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) nameView() string {
	lines := []string{m.styles.header.Render("Project"), m.input.View()}
	if m.nameErr != nil {
		lines = append(lines, m.styles.err.Width(m.width).Render(m.nameErr.Error()))
	}
	return strings.Join(lines, "\n")
}

// defaultJavaIndex returns the position of the metadata's default Java
// version, or 0 if the default is not among the offered versions.
func (m Model) defaultJavaIndex() int {
	return max(slices.Index(m.metadata.JavaVersions, m.metadata.DefaultJavaVersion), 0)
}

func (m Model) javaVersion() string {
	if m.javaCursor < len(m.metadata.JavaVersions) {
		return m.metadata.JavaVersions[m.javaCursor]
	}
	return ""
}

func (m Model) updateJava(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(k, m.keys.Up):
		m.javaCursor = max(m.javaCursor-1, 0)
	case key.Matches(k, m.keys.Down):
		m.javaCursor = min(m.javaCursor+1, len(m.metadata.JavaVersions)-1)
	case key.Matches(k, m.keys.Next):
		m.setScreen(screenSummary)
	case key.Matches(k, m.keys.Back):
		m.setScreen(screenName)
		return m, m.input.Focus()
	case key.Matches(k, m.keys.Quit):
		return m.quit()
	}
	return m, nil
}

func (m Model) javaView() string {
	lines := []string{m.styles.header.Render("Java version")}
	for i, version := range m.metadata.JavaVersions {
		if i == m.javaCursor {
			lines = append(lines, m.styles.selected.Render("> "+version))
		} else {
			lines = append(lines, "  "+version)
		}
	}
	return strings.Join(lines, "\n")
}

func (m Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(k, m.keys.Back):
		m.setScreen(screenJava)
	case key.Matches(k, m.keys.Quit):
		return m.quit()
	}
	return m, nil
}

func (m Model) summaryView() string {
	return strings.Join([]string{
		m.styles.subtle.Render("Project  ") + m.name,
		m.styles.subtle.Render("Java     ") + m.javaVersion(),
	}, "\n")
}
