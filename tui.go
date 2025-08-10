// tui.go
package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/getkin/kin-openapi/openapi3"
)

type ViewState int

const (
	EndpointListView ViewState = iota
	EndpointDetailView
	ResponseView
	ConfigListView
	ConfigEditView
)

type Model struct {
	spec           *openapi3.T
	endpoints      []APIEndpoint
	selectedIndex  int
	viewState      ViewState
	paramInputs    []textinput.Model
	bodyInput      textinput.Model
	response       string
	err            error
	width          int
	height         int
	baseURL        string
	scrollOffset   int
	configManager  *ConfigManager
	configNames    []string
	selectedConfig int
	configInputs   []textinput.Model
	editingConfig  LegacyRequestConfig
}

func NewModel(spec *openapi3.T) Model {
	endpoints := GetEndpoints(spec)

	// Initialize config manager
	configManager, err := NewConfigManager()
	if err != nil {
		// Fallback if config manager fails
		configManager = nil
	}

	// Get base URL from servers or config
	baseURL := ""
	if configManager != nil {
		config := configManager.GetActiveConfig()
		if config.BaseURL != "" {
			baseURL = config.BaseURL
		}
	}

	// Fallback to OpenAPI spec servers
	if baseURL == "" && len(spec.Servers) > 0 {
		baseURL = spec.Servers[0].URL
	}

	return Model{
		spec:          spec,
		endpoints:     endpoints,
		viewState:     EndpointListView,
		baseURL:       baseURL,
		configManager: configManager,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.viewState {
		case EndpointListView:
			return m.updateEndpointList(msg)
		case EndpointDetailView:
			return m.updateEndpointDetail(msg)
		case ResponseView:
			return m.updateResponseView(msg)
		case ConfigListView:
			return m.updateConfigList(msg)
		case ConfigEditView:
			return m.updateConfigEdit(msg)
		}
	}

	return m, nil
}

func (m Model) updateEndpointList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.selectedIndex > 0 {
			m.selectedIndex--
			// Update scroll offset
			if m.selectedIndex < m.scrollOffset {
				m.scrollOffset = m.selectedIndex
			}
		}
	case "down", "j":
		if m.selectedIndex < len(m.endpoints)-1 {
			m.selectedIndex++
			// Update scroll offset
			visibleItems := m.height - 10
			if m.selectedIndex >= m.scrollOffset+visibleItems {
				m.scrollOffset = m.selectedIndex - visibleItems + 1
			}
		}
	case "enter":
		m.viewState = EndpointDetailView
		m.initializeInputs()
	case "c":
		// Open configuration management
		m.viewState = ConfigListView
		m.loadConfigList()
	}
	return m, nil
}

func (m *Model) initializeInputs() {
	endpoint := m.endpoints[m.selectedIndex]

	// Initialize parameter inputs
	m.paramInputs = make([]textinput.Model, len(endpoint.Parameters))
	for i, param := range endpoint.Parameters {
		ti := textinput.New()
		ti.Placeholder = fmt.Sprintf("%s (%s)", param.Name, param.In)
		ti.Width = 50
		if i == 0 {
			ti.Focus()
		}
		m.paramInputs[i] = ti
	}

	// Initialize body input
	if endpoint.RequestBody != nil {
		m.bodyInput = textinput.New()
		m.bodyInput.Placeholder = "Request body (JSON)"
		m.bodyInput.Width = 50
		if len(m.paramInputs) == 0 {
			m.bodyInput.Focus()
		}
	}
}

func (m Model) updateEndpointDetail(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewState = EndpointListView
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		// Move focus between inputs
		m.moveFocus(1)
	case "shift+tab":
		// Move focus backwards
		m.moveFocus(-1)
	case "enter":
		// Send request when pressing enter on the last input
		if m.isLastInputFocused() {
			return m.sendRequest()
		}
		// Otherwise move to next input
		m.moveFocus(1)
	case "ctrl+s":
		// Send request
		return m.sendRequest()
	}

	// Update the focused input
	var cmd tea.Cmd
	for i := range m.paramInputs {
		if m.paramInputs[i].Focused() {
			m.paramInputs[i], cmd = m.paramInputs[i].Update(msg)
			return m, cmd
		}
	}

	if m.endpoints[m.selectedIndex].RequestBody != nil && m.bodyInput.Focused() {
		m.bodyInput, cmd = m.bodyInput.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) moveFocus(direction int) {
	totalInputs := len(m.paramInputs)
	if m.endpoints[m.selectedIndex].RequestBody != nil {
		totalInputs++
	}

	if totalInputs == 0 {
		return
	}

	// Find current focus
	currentFocus := -1
	for i := range m.paramInputs {
		if m.paramInputs[i].Focused() {
			currentFocus = i
			m.paramInputs[i].Blur()
			break
		}
	}

	if currentFocus == -1 && m.endpoints[m.selectedIndex].RequestBody != nil && m.bodyInput.Focused() {
		currentFocus = len(m.paramInputs)
		m.bodyInput.Blur()
	}

	// Calculate new focus
	newFocus := (currentFocus + direction + totalInputs) % totalInputs

	// Set new focus
	if newFocus < len(m.paramInputs) {
		m.paramInputs[newFocus].Focus()
	} else if m.endpoints[m.selectedIndex].RequestBody != nil {
		m.bodyInput.Focus()
	}
}

func (m Model) isLastInputFocused() bool {
	if m.endpoints[m.selectedIndex].RequestBody != nil {
		return m.bodyInput.Focused()
	}
	if len(m.paramInputs) > 0 {
		return m.paramInputs[len(m.paramInputs)-1].Focused()
	}
	return true
}

func (m Model) sendRequest() (tea.Model, tea.Cmd) {
	endpoint := m.endpoints[m.selectedIndex]

	// Build request parameters
	params := make(map[string]string)
	for i, param := range endpoint.Parameters {
		if value := m.paramInputs[i].Value(); value != "" {
			params[param.Name] = value
		}
	}

	// Get request body
	var body string
	if endpoint.RequestBody != nil {
		body = m.bodyInput.Value()
	}

	// Send the request using active configuration
	var response string
	var err error

	if m.configManager != nil {
		config := m.configManager.GetActiveConfig()
		response, err = SendHTTPRequest(config, endpoint, params, body)
	} else {
		// Fallback to basic config
		fallbackConfig := &LegacyRequestConfig{
			BaseURL: m.baseURL,
			Headers: make(map[string]string),
			Cookies: make(map[string]string),
			Timeout: 30,
		}
		response, err = SendHTTPRequest(fallbackConfig, endpoint, params, body)
	}

	if err != nil {
		m.err = err
		m.response = ""
	} else {
		m.err = nil
		m.response = response
	}

	m.viewState = ResponseView
	return m, nil
}

func (m Model) updateResponseView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewState = EndpointDetailView
		return m, nil
	case "ctrl+c", "q":
		return m, tea.Quit
	}
	return m, nil
}

func (m Model) View() string {
	switch m.viewState {
	case EndpointListView:
		return m.renderEndpointList()
	case EndpointDetailView:
		return m.renderEndpointDetail()
	case ResponseView:
		return m.renderResponse()
	case ConfigListView:
		return m.renderConfigList()
	case ConfigEditView:
		return m.renderConfigEdit()
	default:
		return ""
	}
}

func (m Model) renderEndpointList() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	methodStyle := lipgloss.NewStyle().Bold(true)

	var s strings.Builder
	s.WriteString(titleStyle.Render("API Endpoints") + "\n")
	s.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ to navigate, Enter to select, c for configs, q to quit") + "\n")

	// Show active config
	if m.configManager != nil {
		activeConfig := m.configManager.GetActiveConfig()
		s.WriteString(lipgloss.NewStyle().Faint(true).Render(fmt.Sprintf("Active config: %s (%s)", activeConfig.Name, activeConfig.BaseURL)) + "\n\n")
	} else {
		s.WriteString("\n")
	}

	// Calculate visible range
	visibleStart := m.scrollOffset
	visibleEnd := m.scrollOffset + m.height - 10
	if visibleEnd > len(m.endpoints) {
		visibleEnd = len(m.endpoints)
	}

	for i := visibleStart; i < visibleEnd; i++ {
		endpoint := m.endpoints[i]

		method := methodStyle.Copy()
		switch endpoint.Method {
		case "GET":
			method = method.Foreground(lipgloss.Color("42"))
		case "POST":
			method = method.Foreground(lipgloss.Color("33"))
		case "PUT":
			method = method.Foreground(lipgloss.Color("214"))
		case "DELETE":
			method = method.Foreground(lipgloss.Color("196"))
		case "PATCH":
			method = method.Foreground(lipgloss.Color("172"))
		}

		line := fmt.Sprintf("%-8s %s", method.Render(endpoint.Method), endpoint.Path)
		if endpoint.Summary != "" {
			line += lipgloss.NewStyle().Faint(true).Render(" - " + endpoint.Summary)
		}

		if i == m.selectedIndex {
			s.WriteString(selectedStyle.Render(line) + "\n")
		} else {
			s.WriteString(line + "\n")
		}
	}

	// Show scroll indicator
	if m.scrollOffset > 0 || visibleEnd < len(m.endpoints) {
		scrollInfo := fmt.Sprintf("\n[%d-%d of %d]", visibleStart+1, visibleEnd, len(m.endpoints))
		s.WriteString(lipgloss.NewStyle().Faint(true).Render(scrollInfo))
	}

	return s.String()
}

func (m Model) renderEndpointDetail() string {
	endpoint := m.endpoints[m.selectedIndex]

	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	labelStyle := lipgloss.NewStyle().Bold(true)

	var s strings.Builder
	s.WriteString(titleStyle.Render(fmt.Sprintf("%s %s", endpoint.Method, endpoint.Path)) + "\n")

	if endpoint.Description != "" {
		s.WriteString(lipgloss.NewStyle().Faint(true).Render(endpoint.Description) + "\n")
	}

	s.WriteString(lipgloss.NewStyle().Faint(true).Render("Tab to navigate, Ctrl+S to send, Esc to go back") + "\n\n")

	// Render parameters
	if len(endpoint.Parameters) > 0 {
		s.WriteString(labelStyle.Render("Parameters:") + "\n")
		for i, param := range endpoint.Parameters {
			required := ""
			if param.Required {
				required = " *"
			}
			s.WriteString(fmt.Sprintf("  %s (%s)%s: %s\n",
				param.Name,
				param.In,
				required,
				m.paramInputs[i].View()))
			if param.Description != "" {
				s.WriteString(lipgloss.NewStyle().Faint(true).Render("    "+param.Description) + "\n")
			}
		}
		s.WriteString("\n")
	}

	// Render request body
	if endpoint.RequestBody != nil {
		s.WriteString(labelStyle.Render("Request Body:") + "\n")
		s.WriteString("  " + m.bodyInput.View() + "\n")
	}

	return s.String()
}

func (m Model) renderResponse() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	var s strings.Builder
	s.WriteString(titleStyle.Render("Response") + "\n")
	s.WriteString(lipgloss.NewStyle().Faint(true).Render("Esc to go back") + "\n\n")

	if m.err != nil {
		s.WriteString(errorStyle.Render("Error: "+m.err.Error()) + "\n")
	} else {
		s.WriteString(m.response)
	}

	return s.String()
}

// Configuration management methods
func (m *Model) loadConfigList() {
	if m.configManager == nil {
		return
	}
	m.configNames = m.configManager.ListConfigs()
	m.selectedConfig = 0
}

func (m Model) updateConfigList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewState = EndpointListView
		return m, nil
	case "ctrl+c", "q":
		return m, tea.Quit
	case "up", "k":
		if m.selectedConfig > 0 {
			m.selectedConfig--
		}
	case "down", "j":
		if m.selectedConfig < len(m.configNames)-1 {
			m.selectedConfig++
		}
	case "enter":
		// Set as active config
		if len(m.configNames) > 0 {
			configName := m.configNames[m.selectedConfig]
			if m.configManager != nil {
				m.configManager.SetActiveConfig(configName)
				// Update baseURL for display
				activeConfig := m.configManager.GetActiveConfig()
				m.baseURL = activeConfig.BaseURL
			}
		}
		m.viewState = EndpointListView
		return m, nil
	case "n":
		// Create new config
		m.editingConfig = LegacyRequestConfig{
			Name:        "",
			BaseURL:     "",
			Headers:     make(map[string]string),
			Cookies:     make(map[string]string),
			Timeout:     30,
			UserAgent:   "API-Man/1.0",
			ContentType: "application/json",
		}
		m.initializeConfigInputs()
		m.viewState = ConfigEditView
	}
	return m, nil
}

func (m *Model) initializeConfigInputs() {
	inputs := make([]textinput.Model, 6)

	// Name
	inputs[0] = textinput.New()
	inputs[0].Placeholder = "Config name"
	inputs[0].Width = 50
	inputs[0].SetValue(m.editingConfig.Name)
	inputs[0].Focus()

	// Base URL
	inputs[1] = textinput.New()
	inputs[1].Placeholder = "Base URL (e.g., https://api.example.com)"
	inputs[1].Width = 50
	inputs[1].SetValue(m.editingConfig.BaseURL)

	// Authorization
	inputs[2] = textinput.New()
	inputs[2].Placeholder = "Authorization (e.g., Bearer token123)"
	inputs[2].Width = 50
	inputs[2].SetValue(m.editingConfig.Authorization)

	// User Agent
	inputs[3] = textinput.New()
	inputs[3].Placeholder = "User Agent"
	inputs[3].Width = 50
	inputs[3].SetValue(m.editingConfig.UserAgent)

	// Content Type
	inputs[4] = textinput.New()
	inputs[4].Placeholder = "Content Type"
	inputs[4].Width = 50
	inputs[4].SetValue(m.editingConfig.ContentType)

	// Timeout
	inputs[5] = textinput.New()
	inputs[5].Placeholder = "Timeout (seconds)"
	inputs[5].Width = 50
	inputs[5].SetValue(fmt.Sprintf("%d", m.editingConfig.Timeout))

	m.configInputs = inputs
}

func (m Model) updateConfigEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.viewState = ConfigListView
		return m, nil
	case "ctrl+c":
		return m, tea.Quit
	case "tab":
		m.moveConfigFocus(1)
	case "shift+tab":
		m.moveConfigFocus(-1)
	case "enter":
		m.moveConfigFocus(1)
	}

	// Update the focused input
	var cmd tea.Cmd
	for i := range m.configInputs {
		if m.configInputs[i].Focused() {
			m.configInputs[i], cmd = m.configInputs[i].Update(msg)
			return m, cmd
		}
	}

	return m, nil
}

func (m *Model) moveConfigFocus(direction int) {
	if len(m.configInputs) == 0 {
		return
	}

	// Find current focus
	currentFocus := -1
	for i := range m.configInputs {
		if m.configInputs[i].Focused() {
			currentFocus = i
			m.configInputs[i].Blur()
			break
		}
	}

	// Calculate new focus
	newFocus := (currentFocus + direction + len(m.configInputs)) % len(m.configInputs)
	m.configInputs[newFocus].Focus()
}

func (m Model) isLastConfigInputFocused() bool {
	return len(m.configInputs) > 0 && m.configInputs[len(m.configInputs)-1].Focused()
}

func (m Model) renderConfigList() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selectedStyle := lipgloss.NewStyle().Background(lipgloss.Color("62")).Foreground(lipgloss.Color("230"))
	activeStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))

	var s strings.Builder
	s.WriteString(titleStyle.Render("Configurations") + "\n")
	s.WriteString(lipgloss.NewStyle().Faint(true).Render("↑/↓ to navigate, Enter to activate, e to edit, n for new, d to delete, Esc to go back") + "\n\n")

	if m.configManager == nil {
		s.WriteString("Configuration manager not available")
		return s.String()
	}

	activeConfigName := m.configManager.GetActiveConfig().Name

	for _, configName := range m.configNames {
		line := configName
		if configName == activeConfigName {
			line = activeStyle.Render("● " + line)
		} else {
			line = "  " + line
		}
		s.WriteString(selectedStyle.Render(line) + "\n")
	}

	return s.String()
}

func (m Model) renderConfigEdit() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	labelStyle := lipgloss.NewStyle().Bold(true)

	var s strings.Builder
	if m.editingConfig.Name != "" {
		s.WriteString(titleStyle.Render("Edit Configuration: "+m.editingConfig.Name) + "\n")
	} else {
		s.WriteString(titleStyle.Render("New Configuration") + "\n")
	}

	s.WriteString(lipgloss.NewStyle().Faint(true).Render("Tab to navigate, Ctrl+S to save, Esc to cancel") + "\n\n")

	// Configuration fields
	fields := []string{
		"Name:",
		"Base URL:",
		"Authorization:",
		"User Agent:",
		"Content Type:",
		"Timeout (seconds):",
	}

	for i, field := range fields {
		s.WriteString(labelStyle.Render(field) + "\n")
		s.WriteString("  " + m.configInputs[i].View() + "\n\n")
	}

	return s.String()
}
