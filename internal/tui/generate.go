package tui

import (
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
)

func (m Model) request() initializr.Request {
	return initializr.Request{
		Name:         m.name,
		GroupID:      m.groupID,
		PackageName:  initializr.PackageName(m.groupID, m.name),
		JavaVersion:  m.javaVersion(),
		Dependencies: m.selectedDeps(),
	}
}

// generateCmd creates the project in a directory named after it, inside the
// current working directory.
func (m Model) generateCmd() tea.Cmd {
	backend, ctx, req := m.backend, m.ctx, m.request()
	return func() tea.Msg {
		if err := backend.Generate(ctx, req, req.Name); err != nil {
			return generateFailedMsg{err}
		}
		return generatedMsg{}
	}
}

func (m Model) startGenerating() (Model, tea.Cmd) {
	m.setScreen(screenGenerating)
	return m, tea.Batch(m.spinner.Tick, m.generateCmd())
}

func (m Model) updateGenerating(msg tea.Msg) (Model, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && key.Matches(k, m.keys.Quit) {
		return m.quit()
	}
	var cmd tea.Cmd
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

func (m Model) doneView() string {
	return m.styles.result.Render(strings.Join([]string{
		m.styles.success.Render("Project generated successfully."),
		"",
		"Run it:",
		"cd " + m.name,
		"./mvnw spring-boot:run",
	}, "\n"))
}
