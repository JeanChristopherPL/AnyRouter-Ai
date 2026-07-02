package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type appState int

const (
	stateMainMenu    appState = iota
	stateServerLive
	stateConfigMenu
	stateProviderList
	stateProviderForm
	stateServerSettings
	stateProviderHealth
	stateInstallPath
	stateSupportInfo
	stateConfirmQuit
	stateTestEndpoint
)

type inputField struct {
	label string
	value string
	input textinput.Model
}

type tuiModel struct {
	state      appState
	cfg        *Config
	width      int
	height     int
	ready      bool
	quitting   bool

	// Server
	server     *ProxyServer
	serverRunning bool
	reqCount   int64
	serverLogs []string
	serverErr  string

	// Menu cursor
	cursor     int
	menuItems  []string

	// Provider list
	providerCursor int
	providerNames  []string

	// Provider form
	formFields    []inputField
	formFocus     int
	editingProvider string
	formMode      string // "add" or "edit"

	// Settings form
	settingsFields []inputField
	settingsFocus  int

	// Health results
	healthResults []string

	// Version check
	latestVersion  string
	hasUpdate      bool
	updateCheckDone bool

	// Test endpoint
	testProvider   string
	testModel      string
	testResult     string
	testResultTime time.Time

	// Error
	err          error
	statusMsg    string
	statusMsgTime time.Time
}

func initialModel(cfg *Config) tuiModel {
	m := tuiModel{
		state:      stateMainMenu,
		cfg:        cfg,
		serverLogs: []string{},
	}
	m.menuItems = mainMenuItems(false)
	return m
}

func mainMenuItems(running bool) []string {
	if running {
		return []string{
			"Server Dashboard (Running)",
			"Configure Providers",
			"Server Settings",
			"Provider Health Status",
			"Install to PATH",
			"Support the Project",
			"Quit",
		}
	}
	return []string{
		"Start Server",
		"Configure Providers",
		"Server Settings",
		"Provider Health Status",
		"Install to PATH",
		"Support the Project",
		"Quit",
	}
}

// ─── Init ─────────────────────────────────────────────────────────────

func (m tuiModel) Init() tea.Cmd {
	return checkVersionCmd()
}

// ─── Update ───────────────────────────────────────────────────────────

func (m tuiModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		// Global quit
		if msg.String() == "ctrl+c" {
			m.quitting = true
			return m, tea.Quit
		}

		switch m.state {
		case stateMainMenu:
			return m.updateMainMenu(msg)
		case stateServerLive:
			return m.updateServerLive(msg)
		case stateConfigMenu:
			return m.updateConfigMenu(msg)
		case stateProviderList:
			return m.updateProviderList(msg)
		case stateProviderForm:
			return m.updateProviderForm(msg)
		case stateServerSettings:
			return m.updateServerSettings(msg)
		case stateProviderHealth:
			return m.updateProviderHealth(msg)
	case stateInstallPath:
		return m.updateInstallPath(msg)
	case stateSupportInfo:
		return m.updateSupportInfo(msg)
	case stateTestEndpoint:
			return m.updateTestEndpoint(msg)
		case stateConfirmQuit:
			return m.updateConfirmQuit(msg)
		}

	case serverStartedMsg:
		m.serverRunning = true
		m.server = msg.server
		m.menuItems = mainMenuItems(true)
		m.serverLogs = append(m.serverLogs, "Server started on port "+strconv.Itoa(m.cfg.Server.Port))
		m.state = stateServerLive
		m.statusMsg = "Server started successfully!"
		m.statusMsgTime = time.Now()
		return m, nil

	case serverStoppedMsg:
		m.serverRunning = false
		m.server = nil
		m.menuItems = mainMenuItems(false)
		m.serverLogs = append(m.serverLogs, "Server stopped")
		m.state = stateMainMenu
		m.statusMsg = "Server stopped"
		m.statusMsgTime = time.Now()
		return m, nil

	case serverErrorMsg:
		m.serverErr = string(msg)
		m.statusMsg = "Server error: " + string(msg)
		m.statusMsgTime = time.Now()
		return m, nil

	case reqCountMsg:
		atomic.StoreInt64(&m.reqCount, int64(msg))
		return m, nil

	case statusMsg:
		m.statusMsg = string(msg)
		m.statusMsgTime = time.Now()
		return m, nil

	case healthDoneMsg:
		m.healthResults = msg.results
		m.statusMsg = "Health check complete"
		m.statusMsgTime = time.Now()
		return m, nil

	case testDoneMsg:
		m.testResult = msg.result
		m.testResultTime = time.Now()
		m.statusMsg = "Test complete"
		m.statusMsgTime = time.Now()
		return m, nil

	case versionCheckMsg:
		m.updateCheckDone = true
		if msg.hasUpdate {
			m.hasUpdate = true
			m.latestVersion = msg.latest
		}
		return m, nil
	}

	return m, nil
}

// ─── Main Menu ────────────────────────────────────────────────────────

func (m tuiModel) updateMainMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		if m.cursor < len(m.menuItems)-1 {
			m.cursor++
		}
	case "enter", " ":
		return m.selectMenuItem()
	case "q":
		m.quitting = true
		return m, tea.Quit
	case "1", "2", "3", "4", "5", "6", "7":
		idx, _ := strconv.Atoi(msg.String())
		m.cursor = idx - 1
		return m.selectMenuItem()
	}
	return m, nil
}

func (m tuiModel) selectMenuItem() (tea.Model, tea.Cmd) {
	switch m.cursor {
	case 0:
		if m.serverRunning {
			m.state = stateServerLive
		} else {
			return m, startServerCmd(m.cfg)
		}
	case 1:
		m.state = stateProviderList
		m.providerCursor = 0
		m.refreshProviderNames()
	case 2:
		m.state = stateServerSettings
		m.initSettingsForm()
	case 3:
		m.state = stateProviderHealth
		return m, checkHealthCmd(m.cfg)
	case 4:
		m.state = stateInstallPath
	case 5:
		m.state = stateSupportInfo
	case 6:
		if m.serverRunning {
			m.state = stateConfirmQuit
		} else {
			m.quitting = true
			return m, tea.Quit
		}
	}
	return m, nil
}

// ─── Server Live ──────────────────────────────────────────────────────

func (m tuiModel) updateServerLive(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "s", "S":
		return m, stopServerCmd(m.server)
	case "t", "T":
		m.state = stateTestEndpoint
		m.testProvider = ""
		m.testModel = ""
		m.testResult = ""
		m.providerCursor = 0
		m.refreshProviderNames()
		return m, nil
	case "q", "esc":
		m.state = stateMainMenu
	}
	return m, nil
}

// ─── Config Menu ──────────────────────────────────────────────────────

func (m tuiModel) updateConfigMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = stateMainMenu
	}
	return m, nil
}

// ─── Provider List ────────────────────────────────────────────────────

func (m tuiModel) updateProviderList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
	case "down", "j":
		if m.providerCursor < len(m.providerNames)-1 {
			m.providerCursor++
		}
	case "enter", " ":
		if len(m.providerNames) > 0 {
			name := m.providerNames[m.providerCursor]
			prov := m.cfg.Providers[name]
			prov.Enabled = !prov.Enabled
			m.cfg.Providers[name] = prov
			// Also update ProviderConfig.Name field
			p := m.cfg.Providers[name]
			p.Name = name
			m.cfg.Providers[name] = p
		}
	case "e", "E":
		if len(m.providerNames) > 0 {
			m.editingProvider = m.providerNames[m.providerCursor]
			m.formMode = "edit"
			m.initProviderForm(m.editingProvider)
			m.state = stateProviderForm
		}
	case "a", "A":
		m.formMode = "add"
		m.editingProvider = ""
		m.initProviderForm("")
		m.state = stateProviderForm
	case "d", "D":
		if len(m.providerNames) > 0 {
			name := m.providerNames[m.providerCursor]
			m.cfg.DeleteProvider(name)
			m.refreshProviderNames()
			if m.providerCursor >= len(m.providerNames) {
				m.providerCursor = len(m.providerNames) - 1
			}
		}
	case "q", "esc":
		m.state = stateMainMenu
	}
	return m, nil
}

func (m *tuiModel) refreshProviderNames() {
	m.providerNames = nil
	for name := range m.cfg.Providers {
		m.providerNames = append(m.providerNames, name)
	}
}

// ─── Provider Form ────────────────────────────────────────────────────

func (m tuiModel) updateProviderForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.formFocus = (m.formFocus + 1) % len(m.formFields)
		return m, focusInputCmd(&m.formFields, m.formFocus)
	case "shift+tab", "up":
		m.formFocus = (m.formFocus - 1 + len(m.formFields)) % len(m.formFields)
		return m, focusInputCmd(&m.formFields, m.formFocus)
	case "enter":
		return m.saveProviderForm()
	case "esc":
		m.state = stateProviderList
		return m, nil
	}

	// Handle text input
	if m.formFocus >= 0 && m.formFocus < len(m.formFields) {
		var cmd tea.Cmd
		m.formFields[m.formFocus].input, cmd = m.formFields[m.formFocus].input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m tuiModel) saveProviderForm() (tea.Model, tea.Cmd) {
	name := m.formFields[0].input.Value()
	if name == "" {
		m.statusMsg = "Provider name is required"
		m.statusMsgTime = time.Now()
		return m, nil
	}

	// Parse API keys
	apiKeysRaw := m.formFields[2].input.Value()
	var apiKeys []string
	var singleKey string
	if apiKeysRaw != "" {
		keys := splitModels(apiKeysRaw)
		if len(keys) == 1 {
			singleKey = keys[0]
		} else {
			apiKeys = keys
		}
	}

	// Parse extra headers
	extraHeaders := make(map[string]string)
	headersRaw := m.formFields[3].input.Value()
	if headersRaw != "" {
		for _, part := range strings.Split(headersRaw, ",") {
			part = strings.TrimSpace(part)
			if idx := strings.Index(part, ": "); idx != -1 {
				extraHeaders[part[:idx]] = part[idx+2:]
			} else if idx := strings.Index(part, ":"); idx != -1 {
				extraHeaders[part[:idx]] = part[idx+1:]
			}
		}
	}

	timeout := 60
	if t, err := strconv.Atoi(m.formFields[8].input.Value()); err == nil && t > 0 {
		timeout = t
	}
	maxRetries := 3
	if r, err := strconv.Atoi(m.formFields[9].input.Value()); err == nil && r >= 0 {
		maxRetries = r
	}
	maxRPM := 0
	if r, err := strconv.Atoi(m.formFields[10].input.Value()); err == nil && r > 0 {
		maxRPM = r
	}
	maxTPM := 0
	if t, err := strconv.Atoi(m.formFields[11].input.Value()); err == nil && t > 0 {
		maxTPM = t
	}
	cooldown := 30
	if c, err := strconv.Atoi(m.formFields[12].input.Value()); err == nil && c > 0 {
		cooldown = c
	}

	prov := ProviderConfig{
		Enabled:        true,
		Name:           name,
		BaseURL:        m.formFields[1].input.Value(),
		APIKey:         singleKey,
		APIKeys:        apiKeys,
		ExtraHeaders:   extraHeaders,
		Format:         m.formFields[4].input.Value(),
		Models:         splitModels(m.formFields[5].input.Value()),
		ModelFallbacks: splitModels(m.formFields[6].input.Value()),
		Fallbacks:      splitModels(m.formFields[7].input.Value()),
		TimeoutSec:     timeout,
		MaxRetries:     maxRetries,
		MaxRPM:         maxRPM,
		MaxTPM:         maxTPM,
		CooldownSec:    cooldown,
		Custom:         true,
	}

	// If editing, remove old entry first
	if m.formMode == "edit" && m.editingProvider != "" && m.editingProvider != name {
		m.cfg.DeleteProvider(m.editingProvider)
	}

	m.cfg.Providers[name] = prov

	// Add model route for the custom provider
	hasRoute := false
	for _, r := range m.cfg.Models {
		if r.Provider == name {
			hasRoute = true
			break
		}
	}
	if !hasRoute {
		m.cfg.Models = append(m.cfg.Models, ModelRoute{Pattern: name + "-", Provider: name})
	}

	m.cfg.Save()
	m.refreshProviderNames()
	m.state = stateProviderList
	m.statusMsg = "Provider '" + name + "' saved"
	m.statusMsgTime = time.Now()
	return m, nil
}

func (m *tuiModel) initProviderForm(name string) {
	m.formFields = nil
	m.formFocus = 0

	prov := m.cfg.Providers[name]

	apiKeysStr := strings.Join(prov.APIKeys, ", ")
	if prov.APIKey != "" && len(prov.APIKeys) == 0 {
		apiKeysStr = prov.APIKey
	}

	var extraHeadersStr string
	for k, v := range prov.ExtraHeaders {
		if extraHeadersStr != "" {
			extraHeadersStr += ", "
		}
		extraHeadersStr += k + ": " + v
	}

	timeoutStr := ""
	if prov.TimeoutSec > 0 {
		timeoutStr = fmt.Sprintf("%d", prov.TimeoutSec)
	}
	maxRetriesStr := ""
	if prov.MaxRetries > 0 {
		maxRetriesStr = fmt.Sprintf("%d", prov.MaxRetries)
	}
	maxRPMStr := ""
	if prov.MaxRPM > 0 {
		maxRPMStr = fmt.Sprintf("%d", prov.MaxRPM)
	}
	maxTPMStr := ""
	if prov.MaxTPM > 0 {
		maxTPMStr = fmt.Sprintf("%d", prov.MaxTPM)
	}
	cooldownStr := ""
	if prov.CooldownSec > 0 {
		cooldownStr = fmt.Sprintf("%d", prov.CooldownSec)
	}

	fields := []struct {
		label string
		value string
		ph    string
	}{
		{"Name", name, "e.g. my-provider"},
		{"Base URL", prov.BaseURL, "https://api.example.com/v1"},
		{"API Keys (comma-separated)", apiKeysStr, "sk-key1, sk-key2"},
		{"Extra Headers (key:val, ...)", extraHeadersStr, "Authorization: Bearer xxx"},
		{"Format", prov.Format, "openai, anthropic, gemini, cohere"},
		{"Models (comma-separated)", strings.Join(prov.Models, ", "), "gpt-4o, gpt-4o-mini"},
		{"Model Fallbacks (comma-separated)", strings.Join(prov.ModelFallbacks, ", "), "gpt-4o-mini"},
		{"Fallback Providers (comma-separated)", strings.Join(prov.Fallbacks, ", "), "azure, openrouter"},
		{"Timeout (seconds)", timeoutStr, "60"},
		{"Max Retries", maxRetriesStr, "3"},
		{"Max RPM", maxRPMStr, "500"},
		{"Max TPM", maxTPMStr, "100000"},
		{"Cooldown (seconds)", cooldownStr, "30"},
	}

	for _, f := range fields {
		ti := textinput.New()
		ti.Placeholder = f.ph
		ti.SetValue(f.value)
		ti.Width = 50
		m.formFields = append(m.formFields, inputField{
			label: f.label,
			value: f.value,
			input: ti,
		})
	}

	if len(m.formFields) > 0 {
		m.formFields[0].input.Focus()
	}
}

// ─── Server Settings ──────────────────────────────────────────────────

func (m *tuiModel) initSettingsForm() {
	m.settingsFields = nil
	m.settingsFocus = 0

	port := strconv.Itoa(m.cfg.Server.Port)
	items := []struct {
		label string
		value string
		ph    string
	}{
		{"Port", port, "9876"},
		{"Default Format", m.cfg.DefaultFormat, "openai"},
		{"Log Level", m.cfg.Server.LogLevel, "info"},
		{"Host", m.cfg.Server.Host, "127.0.0.1"},
		{"Config Path", m.cfg.ConfigFile, "anyrouter.yaml"},
	}

	for _, it := range items {
		ti := textinput.New()
		ti.Placeholder = it.ph
		ti.SetValue(it.value)
		ti.Width = 40
		m.settingsFields = append(m.settingsFields, inputField{
			label: it.label,
			value: it.value,
			input: ti,
		})
	}
	if len(m.settingsFields) > 0 {
		m.settingsFields[0].input.Focus()
	}
}

func (m tuiModel) updateServerSettings(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab", "down":
		m.settingsFocus = (m.settingsFocus + 1) % len(m.settingsFields)
		return m, focusInputSettingsCmd(&m.settingsFields, m.settingsFocus)
	case "shift+tab", "up":
		m.settingsFocus = (m.settingsFocus - 1 + len(m.settingsFields)) % len(m.settingsFields)
		return m, focusInputSettingsCmd(&m.settingsFields, m.settingsFocus)
	case "enter":
		port, _ := strconv.Atoi(m.settingsFields[0].input.Value())
		if port > 0 {
			m.cfg.Server.Port = port
		}
		m.cfg.DefaultFormat = m.settingsFields[1].input.Value()
		m.cfg.Server.LogLevel = m.settingsFields[2].input.Value()
		m.cfg.Server.Host = m.settingsFields[3].input.Value()
		m.cfg.Save()
		m.state = stateMainMenu
		m.statusMsg = "Settings saved (port: " + strconv.Itoa(m.cfg.Server.Port) + ")"
		m.statusMsgTime = time.Now()
		return m, nil
	case "esc":
		m.state = stateMainMenu
		return m, nil
	}

	if m.settingsFocus >= 0 && m.settingsFocus < len(m.settingsFields) {
		var cmd tea.Cmd
		m.settingsFields[m.settingsFocus].input, cmd = m.settingsFields[m.settingsFocus].input.Update(msg)
		return m, cmd
	}

	return m, nil
}

// ─── Provider Health ──────────────────────────────────────────────────

func (m tuiModel) updateProviderHealth(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "r", "R":
		return m, checkHealthCmd(m.cfg)
	case "q", "esc":
		m.state = stateMainMenu
	}
	return m, nil
}

// ─── Install PATH ─────────────────────────────────────────────────────

func (m tuiModel) updateInstallPath(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "i", "I":
		return m, installPathCmd()
	case "q", "esc":
		m.state = stateMainMenu
	}
	return m, nil
}

// ─── Test Endpoint ──────────────────────────────────────────────────

func (m tuiModel) updateTestEndpoint(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.providerCursor > 0 {
			m.providerCursor--
		}
	case "down", "j":
		if m.providerCursor < len(m.providerNames)-1 {
			m.providerCursor++
		}
	case "enter", " ":
		if len(m.providerNames) > 0 {
			name := m.providerNames[m.providerCursor]
			return m, testProviderCmd(m.cfg, name, "")
		}
	case "r", "R":
		if m.testResult != "" {
			m.testResult = ""
		}
	case "q", "esc":
		m.state = stateServerLive
	}
	return m, nil
}

// ─── Support Info ────────────────────────────────────────────────────

func (m tuiModel) updateSupportInfo(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "esc":
		m.state = stateMainMenu
	}
	return m, nil
}

// ─── Confirm Quit ─────────────────────────────────────────────────────

func (m tuiModel) updateConfirmQuit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		if m.serverRunning && m.server != nil {
			m.server.Shutdown()
		}
		m.quitting = true
		return m, tea.Quit
	case "n", "N", "esc":
		m.state = stateMainMenu
	}
	return m, nil
}

// ─── Commands ─────────────────────────────────────────────────────────

type serverStartedMsg struct{ server *ProxyServer }
type serverStoppedMsg struct{}
type serverErrorMsg string
type reqCountMsg int64
type statusMsg string
type healthDoneMsg struct{ results []string }
type testDoneMsg struct{ result string }
type versionCheckMsg struct{ latest string; hasUpdate bool; err string }

func startServerCmd(cfg *Config) tea.Cmd {
	return func() tea.Msg {
		server := NewProxyServer(cfg)
		go func() {
			if err := server.Start(); err != nil {
				server.err = err
			}
		}()
		time.Sleep(100 * time.Millisecond)
		return serverStartedMsg{server: server}
	}
}

func stopServerCmd(server *ProxyServer) tea.Cmd {
	return func() tea.Msg {
		if server != nil {
			server.Shutdown()
		}
		return serverStoppedMsg{}
	}
}

func checkHealthCmd(cfg *Config) tea.Cmd {
	return func() tea.Msg {
		var results []string
		for name, prov := range cfg.Providers {
			if !prov.Enabled {
				continue
			}
			ok, msg := CheckProviderHealth(&prov)
			status := "✓"
			if !ok {
				status = "✗"
			}
			results = append(results, fmt.Sprintf("%s %s: %s", status, name, msg))
		}
		return healthDoneMsg{results: results}
	}
}

func testProviderCmd(cfg *Config, providerName string, model string) tea.Cmd {
	return func() tea.Msg {
		prov, ok := cfg.Providers[providerName]
		if !ok {
			return testDoneMsg{result: "Provider not found: " + providerName}
		}

		testModel := model
		if testModel == "" {
			if len(prov.Models) > 0 {
				testModel = prov.Models[0]
			} else {
				testModel = providerName + "-test"
			}
		}

		ok, msg := CheckProviderHealth(&prov)
		status := "OK"
		if !ok {
			status = "FAIL"
		}
		return testDoneMsg{result: fmt.Sprintf("Provider: %s\nModel: %s\nStatus: %s\nDetail: %s", providerName, testModel, status, msg)}
	}
}

func checkVersionCmd() tea.Cmd {
	return func() tea.Msg {
		versionMsg := versionCheckMsg{latest: Version, hasUpdate: false}
		resp, err := http.Get("https://api.github.com/repos/anyrouter/cli/releases/latest")
		if err != nil {
			versionMsg.err = "could not check: " + err.Error()
			return versionMsg
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		var release struct {
			TagName string `json:"tag_name"`
		}
		if err := json.Unmarshal(body, &release); err != nil {
			versionMsg.err = "parse error"
			return versionMsg
		}
		latest := strings.TrimPrefix(release.TagName, "v")
		current := strings.TrimPrefix(Version, "v")
		versionMsg.latest = latest
		if compareVersions(latest, current) > 0 {
			versionMsg.hasUpdate = true
		}
		return versionMsg
	}
}

func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}
	for i := 0; i < maxLen; i++ {
		var va, vb int
		if i < len(partsA) {
			va, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			vb, _ = strconv.Atoi(partsB[i])
		}
		if va > vb {
			return 1
		}
		if va < vb {
			return -1
		}
	}
	return 0
}

func installPathCmd() tea.Cmd {
	return func() tea.Msg {
		exe, err := os.Executable()
		if err != nil {
			return statusMsg("Error finding executable: " + err.Error())
		}
		dir := filepath.Dir(exe)

		// Check if already in PATH
		pathEnv := os.Getenv("PATH")
		if strings.Contains(strings.ToLower(pathEnv), strings.ToLower(dir)) {
			return statusMsg("Directory is already in PATH: " + dir)
		}

		// Add to PATH via PowerShell for current session
		cmd := exec.Command("powershell", "-Command",
			fmt.Sprintf(`[Environment]::SetEnvironmentVariable("PATH", [Environment]::GetEnvironmentVariable("PATH", "User") + ";%s", "User")`, dir))
		if err := cmd.Run(); err != nil {
			return statusMsg("Could not add to PATH. Try running as Administrator.\nManually add to PATH: " + dir)
		}

		return statusMsg("Added to PATH! Restart your terminal or run:\n  $env:PATH += \";" + dir + "\"")
	}
}

func focusInputCmd(fields *[]inputField, idx int) tea.Cmd {
	for i := range *fields {
		if i == idx {
			(*fields)[i].input.Focus()
		} else {
			(*fields)[i].input.Blur()
		}
	}
	return nil
}

func focusInputSettingsCmd(fields *[]inputField, idx int) tea.Cmd {
	for i := range *fields {
		if i == idx {
			(*fields)[i].input.Focus()
		} else {
			(*fields)[i].input.Blur()
		}
	}
	return nil
}

// ─── Helpers ──────────────────────────────────────────────────────────

func splitModels(s string) []string {
	if s == "" {
		return nil
	}
	var res []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			res = append(res, p)
		}
	}
	return res
}

// ─── View ─────────────────────────────────────────────────────────────

func (m tuiModel) View() string {
	if !m.ready {
		return "Loading..."
	}

	switch m.state {
	case stateMainMenu:
		return m.viewMainMenu()
	case stateServerLive:
		return m.viewServerLive()
	case stateConfigMenu:
		return m.viewConfigMenu()
	case stateProviderList:
		return m.viewProviderList()
	case stateProviderForm:
		return m.viewProviderForm()
	case stateServerSettings:
		return m.viewServerSettings()
	case stateProviderHealth:
		return m.viewProviderHealth()
	case stateInstallPath:
		return m.viewInstallPath()
	case stateSupportInfo:
		return m.viewSupportInfo()
	case stateConfirmQuit:
		return m.viewConfirmQuit()
	case stateTestEndpoint:
		return m.viewTestEndpoint()
	}
	return "Unknown state"
}

func (m tuiModel) viewMainMenu() string {
	var b strings.Builder

	b.WriteString(RenderBanner())
	b.WriteString("\n")

	// Status message
	if m.statusMsg != "" && time.Since(m.statusMsgTime) < 5*time.Second {
		b.WriteString(styleInfo.Render("  ℹ " + m.statusMsg))
		b.WriteString("\n\n")
	}

	if m.serverRunning {
		b.WriteString(styleOk.Render("  ● Server running on port " + strconv.Itoa(m.cfg.Server.Port)))
		b.WriteString("\n\n")
	}

	if m.hasUpdate {
		b.WriteString(styleWarn.Render("  ⬆ Update available: v" + m.latestVersion + " (current: v" + Version + ")"))
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  Run: powershell -c \"irm https://anyrouter.planixx.com/scripts/install.ps1 | iex\""))
		b.WriteString("\n\n")
	}

	// Menu
	for i, item := range m.menuItems {
		prefix := "  "
		if i == m.cursor {
			prefix = styleCyan.Render("  ▸ ")
		} else {
			prefix = "    "
		}

		num := styleDim.Render(fmt.Sprintf("[%d]", i+1))
		text := item
		if i == m.cursor {
			text = styleCyan.Render(item)
		}

		b.WriteString(fmt.Sprintf("%s%s %s\n", prefix, num, text))
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑/↓] Navigate  [Enter] Select  [q] Quit  [1-6] Quick select"))
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewServerLive() string {
	var b strings.Builder

	b.WriteString(RenderMiniBanner())
	b.WriteString("\n")

	// Server status box
	statusColor := styleOk
	runningText := "● Running"
	if !m.serverRunning {
		statusColor = styleRed
		runningText = "○ Stopped"
	}

	portLine := styleInfo.Render(fmt.Sprintf("  Port: %d", m.cfg.Server.Port))
	statusLine := statusColor.Render(fmt.Sprintf("  Status: %s", runningText))
	uptime := time.Now().Sub(time.Now()) // placeholder
	_ = uptime

	b.WriteString(lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Width(50).
		Render(fmt.Sprintf(
			"%s\n%s\n%s\n%s",
			styleCyan.Render(fmt.Sprintf("  📡 http://127.0.0.1:%d", m.cfg.Server.Port)),
			statusLine,
			portLine,
			styleDim.Render(fmt.Sprintf("  Requests: %d", atomic.LoadInt64(&m.reqCount))),
		)))
	b.WriteString("\n\n")

	// Recent logs
	b.WriteString(styleDim.Render("  ── Recent Activity ──"))
	b.WriteString("\n")
	start := 0
	if len(m.serverLogs) > 5 {
		start = len(m.serverLogs) - 5
	}
	for _, log := range m.serverLogs[start:] {
		b.WriteString(styleDim.Render("  " + log))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [s] Stop Server  [t] Test Endpoint  [q] Back to Menu"))
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewConfigMenu() string {
	return styleYellow.Render("Configuration") + "\n\n" +
		styleDim.Render("Press [q] to go back") + "\n"
}

func (m tuiModel) viewProviderList() string {
	var b strings.Builder

	b.WriteString(styleCyan.Render("  ⚙ Provider Configuration"))
	b.WriteString("\n\n")

	if len(m.providerNames) == 0 {
		b.WriteString(styleDim.Render("  No providers configured. Press [a] to add one."))
		b.WriteString("\n\n")
	}

	for i, name := range m.providerNames {
		prov := m.cfg.Providers[name]
		cursor := "  "
		if i == m.providerCursor {
			cursor = styleCyan.Render("  ▸ ")
		}

		status := styleRed.Render("●")
		if prov.Enabled {
			status = styleOk.Render("●")
		}

		format := styleDim.Render("(" + prov.Format + ")")
		models := ""
		if len(prov.Models) > 0 {
			models = styleDim.Render(" " + strings.Join(prov.Models, ", "))
		}
		custom := ""
		if prov.Custom {
			custom = styleWarn.Render(" [custom]")
		}

		fallbacks := ""
		if len(prov.Fallbacks) > 0 {
			fallbacks = styleDim.Render(" → " + strings.Join(prov.Fallbacks, ", "))
		}

		line := fmt.Sprintf("%s%s %s %s%s%s%s", cursor, status, styleBold.Render(name), format, custom, models, fallbacks)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [↑/↓] Navigate  [Space/Enter] Toggle  [e] Edit  [a] Add  [d] Delete  [q] Back"))
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewProviderForm() string {
	var b strings.Builder

	title := "Add Provider"
	if m.formMode == "edit" {
		title = "Edit Provider: " + m.editingProvider
	}
	b.WriteString(styleCyan.Render("  " + title))
	b.WriteString("\n\n")

	labels := []string{"Name", "Base URL", "API Keys", "Extra Headers", "Format", "Models", "Model Fallbacks", "Fallback Providers", "Timeout (s)", "Max Retries", "Max RPM", "Max TPM", "Cooldown (s)"}

	for i, f := range m.formFields {
		label := labels[i]
		cursor := "  "
		if i == m.formFocus {
			cursor = styleCyan.Render("  ▸ ")
		} else {
			cursor = "    "
		}
		b.WriteString(fmt.Sprintf("%s%s:\n", cursor, styleBold.Render(label)))
		b.WriteString(fmt.Sprintf("    %s\n", f.input.View()))
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [Tab] Next field  [Enter] Save  [Esc] Cancel"))
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewServerSettings() string {
	var b strings.Builder

	b.WriteString(styleCyan.Render("  ⚙ Server Settings"))
	b.WriteString("\n\n")

	labels := []string{"Port", "Default Format", "Log Level", "Host", "Config Path"}

	for i, f := range m.settingsFields {
		cursor := "  "
		if i == m.settingsFocus {
			cursor = styleCyan.Render("  ▸ ")
		} else {
			cursor = "    "
		}
		b.WriteString(fmt.Sprintf("%s%s:\n", cursor, styleBold.Render(labels[i])))
		b.WriteString(fmt.Sprintf("    %s\n", f.input.View()))
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [Tab] Next field  [Enter] Save  [Esc] Cancel"))
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewProviderHealth() string {
	var b strings.Builder

	b.WriteString(styleCyan.Render("  📊 Provider Health Status"))
	b.WriteString("\n\n")

	if len(m.healthResults) == 0 {
		b.WriteString(styleDim.Render("  Checking..."))
	} else {
		for _, r := range m.healthResults {
			status := styleRed.Render("  ✗")
			if strings.HasPrefix(r, "✓") {
				status = styleOk.Render("  ✓")
			}
			detail := strings.TrimSpace(r[2:])
			parts := strings.SplitN(detail, ": ", 2)
			if len(parts) == 2 {
				b.WriteString(fmt.Sprintf("%s %s: %s\n", status, styleBold.Render(parts[0]), styleDim.Render(parts[1])))
			} else {
				b.WriteString(fmt.Sprintf("%s %s\n", status, detail))
			}
		}
	}

	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [r] Refresh  [q] Back"))
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewInstallPath() string {
	var b strings.Builder

	b.WriteString(styleCyan.Render("  📦 Install to PATH"))
	b.WriteString("\n\n")

	exe, err := os.Executable()
	dir := ""
	if err == nil {
		dir = filepath.Dir(exe)
	}

	b.WriteString(styleInfo.Render("  Binary location:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  %s\n", dir))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Press [i] to automatically add this directory"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  to your PATH (User environment variable)."))
	b.WriteString("\n\n")
	b.WriteString(styleWarn.Render("  ⚠ Note: You may need to run as Administrator"))
	b.WriteString("\n")
	b.WriteString(styleWarn.Render("    for the PATH change to take effect."))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  [i] Install to PATH  [q] Back"))
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewTestEndpoint() string {
	var b strings.Builder

	b.WriteString(styleCyan.Render("  🧪 Test Provider Endpoint"))
	b.WriteString("\n\n")
	b.WriteString(styleDim.Render("  Select a provider and press [Enter] to test its endpoint."))
	b.WriteString("\n\n")

	if len(m.providerNames) == 0 {
		b.WriteString(styleDim.Render("  No providers configured."))
		b.WriteString("\n\n")
	} else {
		for i, name := range m.providerNames {
			prov := m.cfg.Providers[name]
			cursor := "  "
			if i == m.providerCursor {
				cursor = styleCyan.Render("  ▸ ")
			}

			status := styleRed.Render("●")
			if prov.Enabled {
				status = styleOk.Render("●")
			}

			format := styleDim.Render("(" + prov.Format + ")")
			models := ""
			if len(prov.Models) > 0 {
				models = styleDim.Render(" " + strings.Join(prov.Models, ", "))
			}

			b.WriteString(fmt.Sprintf("%s%s %s %s%s\n", cursor, status, styleBold.Render(name), format, models))
		}
	}

	if m.testResult != "" {
		b.WriteString("\n")
		b.WriteString(styleDivider.Render("  ── Result ──"))
		b.WriteString("\n")
		for _, line := range strings.Split(m.testResult, "\n") {
			status := strings.TrimSpace(line)
			if strings.HasPrefix(status, "Status: OK") || strings.Contains(status, "ok") {
				b.WriteString(styleOk.Render("  " + line))
			} else if strings.HasPrefix(status, "Status: FAIL") {
				b.WriteString(styleRed.Render("  " + line))
			} else {
				b.WriteString(styleDim.Render("  " + line))
			}
			b.WriteString("\n")
		}
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  [r] Reset Result  [q] Back"))
	} else {
		b.WriteString("\n")
		b.WriteString(styleDim.Render("  [↑/↓] Navigate  [Enter] Test  [q] Back"))
	}
	b.WriteString("\n")

	return b.String()
}

func (m tuiModel) viewSupportInfo() string {
	var b strings.Builder
	b.WriteString(styleCyan.Render("  Support AnyRouter"))
	b.WriteString("\n\n")
	b.WriteString("  AnyRouter is open-source and free to use.\n")
	b.WriteString("  If you find this tool useful, consider supporting the project.\n")
	b.WriteString("\n")
	b.WriteString(styleBold.Render("  Ways to support:\n"))
	b.WriteString("\n")
	b.WriteString("    GitHub Sponsors  ->  https://github.com/sponsors/anyrouter\n")
	b.WriteString("    Buy Me a Coffee  ->  https://ko-fi.com/anyrouter\n")
	b.WriteString("    PayPal           ->  https://paypal.me/anyrouter\n")
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  Your support helps maintain and improve AnyRouter.\n"))
	b.WriteString("\n")
	b.WriteString(styleDim.Render("  [q] Back"))
	b.WriteString("\n")
	return b.String()
}

func (m tuiModel) viewConfirmQuit() string {
	return styleYellow.Render("  Are you sure you want to quit?") + "\n\n" +
		styleDim.Render("  [y] Yes  [n] No") + "\n"
}

// should be used by the server to report request count
type serverStats struct {
	Requests int64
}
