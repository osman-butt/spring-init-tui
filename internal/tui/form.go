package tui

import (
	"cmp"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

// The name becomes both the Maven artifactId and the project directory.
var namePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// The group becomes the Maven groupId and the start of the default Java
// package, so it has to be made of valid package segments, like the package
// itself.
var packagePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// fallbackGroup is used when the metadata does not name a default group.
const fallbackGroup = "com.example"

const examplePackage = fallbackGroup + ".demo"

func newNameInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = "demo"
	input.CharLimit = 64
	return input
}

func newGroupInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = fallbackGroup
	input.CharLimit = 100
	return input
}

func newPackageInput() textinput.Model {
	input := textinput.New()
	input.Placeholder = examplePackage
	input.CharLimit = 200
	return input
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func (m Model) validateName(name string) error {
	switch {
	case name == "":
		return errors.New("enter an artifact name")
	case !namePattern.MatchString(name):
		return errors.New("use letters, digits, '.', '-' and '_' only")
	case m.exists(name):
		return fmt.Errorf("%q already exists in this directory", name)
	}
	return nil
}

func (m Model) updateName(msg tea.Msg) (Model, tea.Cmd) {
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
			m.setScreen(screenGroup)
			return m, m.group.Focus()
		}
		m.nameErr = nil
	}

	// Everything else, including "q", belongs to the text input.
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func validateGroup(group string) error {
	switch {
	case group == "":
		return errors.New("enter a group, for example " + fallbackGroup)
	case !packagePattern.MatchString(group):
		return errors.New("use dot-separated names made of letters, digits and '_', for example " + fallbackGroup)
	}
	return nil
}

func (m Model) updateGroup(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, m.keys.Back):
			m.group.Blur()
			m.setScreen(screenName)
			return m, m.input.Focus()
		case key.Matches(k, m.keys.Next):
			group := strings.TrimSpace(m.group.Value())
			if m.groupErr = validateGroup(group); m.groupErr != nil {
				return m, nil
			}
			m.groupID = group
			m.group.Blur()
			m.prefillPackage()
			m.setScreen(screenPackage)
			return m, m.pkg.Focus()
		}
		m.groupErr = nil
	}

	var cmd tea.Cmd
	m.group, cmd = m.group.Update(msg)
	return m, cmd
}

// prefillPackage offers the package derived from the group and the name,
// unless the user has typed a package of their own.
func (m *Model) prefillPackage() {
	derived := initializr.PackageName(m.groupID, m.name)
	if current := m.pkg.Value(); current == "" || current == m.packageDefault {
		m.pkg.SetValue(derived)
		m.pkg.CursorEnd()
	}
	m.packageDefault = derived
}

func validatePackage(name string) error {
	switch {
	case name == "":
		return errors.New("enter a package name, for example " + examplePackage)
	case !packagePattern.MatchString(name):
		return errors.New("use dot-separated names made of letters, digits and '_', for example " + examplePackage)
	}
	return nil
}

func (m Model) updatePackage(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch {
		case key.Matches(k, m.keys.Back):
			m.pkg.Blur()
			m.setScreen(screenGroup)
			return m, m.group.Focus()
		case key.Matches(k, m.keys.Next):
			name := strings.TrimSpace(m.pkg.Value())
			if m.packageErr = validatePackage(name); m.packageErr != nil {
				return m, nil
			}
			m.packageName = name
			m.pkg.Blur()
			m.setScreen(screenBoot)
			return m, nil
		}
		m.packageErr = nil
	}

	var cmd tea.Cmd
	m.pkg, cmd = m.pkg.Update(msg)
	return m, cmd
}

func (m Model) packageView() string {
	lines := []string{m.styles.header.Render("Package name"), m.pkg.View()}
	if m.packageErr != nil {
		lines = append(lines, m.styles.err.Width(m.width).Render(m.packageErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m Model) groupView() string {
	lines := []string{m.styles.header.Render("Group"), m.group.View()}
	if m.groupErr != nil {
		lines = append(lines, m.styles.err.Width(m.width).Render(m.groupErr.Error()))
	}
	return strings.Join(lines, "\n")
}

func (m Model) nameView() string {
	lines := []string{m.styles.header.Render("Artifact"), m.input.View()}
	if m.nameErr != nil {
		lines = append(lines, m.styles.err.Width(m.width).Render(m.nameErr.Error()))
	}
	return strings.Join(lines, "\n")
}

// choicesView lists the options of a pick-one screen and marks the one under
// the cursor.
func (m Model) choicesView(header string, options []string, cursor int) string {
	lines := []string{m.styles.header.Render(header)}
	for i, option := range options {
		if i == cursor {
			lines = append(lines, m.styles.selected.Render("> "+option))
		} else {
			lines = append(lines, "  "+option)
		}
	}
	return strings.Join(lines, "\n")
}

// defaultBootIndex returns the position of the metadata's default Spring
// Boot version, or 0 if the default is not among the offered versions.
func (m Model) defaultBootIndex() int {
	return max(slices.IndexFunc(m.metadata.BootVersions, func(v initializr.BootVersion) bool {
		return v.ID == m.metadata.DefaultBootVersion
	}), 0)
}

// boot returns the chosen Spring Boot version. It is empty when the metadata
// offers none, which leaves the choice to Initializr.
func (m Model) boot() initializr.BootVersion {
	if m.bootCursor < len(m.metadata.BootVersions) {
		return m.metadata.BootVersions[m.bootCursor]
	}
	return initializr.BootVersion{}
}

// bootLabel is how a Spring Boot version is shown.
func bootLabel(v initializr.BootVersion) string {
	return cmp.Or(v.Name, v.ID)
}

func (m Model) updateBoot(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(k, m.keys.Up):
		m.bootCursor = max(m.bootCursor-1, 0)
	case key.Matches(k, m.keys.Down):
		m.bootCursor = min(m.bootCursor+1, len(m.metadata.BootVersions)-1)
	case key.Matches(k, m.keys.Next):
		// The version decides which dependencies can be offered.
		cmd := m.showAvailableDeps()
		m.setScreen(screenJava)
		return m, cmd
	case key.Matches(k, m.keys.Back):
		m.setScreen(screenPackage)
		return m, m.pkg.Focus()
	case key.Matches(k, m.keys.Quit):
		return m.quit()
	}
	return m, nil
}

func (m Model) bootView() string {
	options := make([]string, len(m.metadata.BootVersions))
	for i, v := range m.metadata.BootVersions {
		options[i] = bootLabel(v)
	}
	return m.choicesView("Spring Boot version", options, m.bootCursor)
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

func (m Model) updateJava(msg tea.Msg) (Model, tea.Cmd) {
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
		m.setScreen(screenDeps)
		return m, m.search.Focus()
	case key.Matches(k, m.keys.Back):
		m.setScreen(screenBoot)
	case key.Matches(k, m.keys.Quit):
		return m.quit()
	}
	return m, nil
}

func (m Model) javaView() string {
	return m.choicesView("Java version", m.metadata.JavaVersions, m.javaCursor)
}

func (m Model) updateConfirm(msg tea.Msg) (Model, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(k, m.keys.Switch):
		m.confirmYes = !m.confirmYes
	case key.Matches(k, m.keys.Yes):
		return m.startGenerating()
	case key.Matches(k, m.keys.Confirm):
		if m.confirmYes {
			return m.startGenerating()
		}
		return m.quit()
	case key.Matches(k, m.keys.No), key.Matches(k, m.keys.Quit):
		return m.quit()
	case key.Matches(k, m.keys.Back):
		m.setScreen(screenDeps)
		return m, m.search.Focus()
	}
	return m, nil
}

func (m Model) confirmView() string {
	yes, no := m.styles.button, m.styles.activeButton
	if m.confirmYes {
		yes, no = no, yes
	}
	return strings.Join([]string{
		m.summaryView(),
		"",
		m.styles.header.Render("Generate " + m.name + "?"),
		yes.Render("Yes") + " " + no.Render("No"),
	}, "\n")
}

func (m Model) summaryView() string {
	deps := "none"
	if ids := m.selectedDeps(); len(ids) > 0 {
		deps = strings.Join(ids, ", ")
	}
	label := m.styles.subtle.Render
	return strings.Join([]string{
		label("Artifact      ") + m.name,
		label("Group         ") + m.groupID,
		label("Package       ") + m.packageName,
		label("Spring Boot   ") + bootLabel(m.boot()),
		label("Java          ") + m.javaVersion(),
		// Wrap a long list under its own column instead of cutting it off.
		lipgloss.JoinHorizontal(lipgloss.Top, label("Dependencies  "),
			lipgloss.NewStyle().Width(max(m.width-14, 10)).Render(deps)),
	}, "\n")
}
