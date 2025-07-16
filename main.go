package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AIResource struct {
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Type        string    `json:"type"` // template, prompt, context, dataset
	Tags        []string  `json:"tags"`
	Description string    `json:"description"`
	Size        int64     `json:"size"`
	ModTime     time.Time `json:"mod_time"`
	IsTemplate  bool      `json:"is_template"`
}

type Config struct {
	TemplatesDir string   `json:"templates_dir"`
	ResourceDirs []string `json:"resource_dirs"`
	FileTypes    []string `json:"file_types"`
}

type ViewMode int

const (
	ViewTable ViewMode = iota
	ViewDetails
	ViewCreate
)

type statusMsg struct {
	message string
}

func showStatus(msg string) tea.Cmd {
	return func() tea.Msg {
		return statusMsg{message: msg}
	}
}

type model struct {
	config       Config
	resources    []AIResource
	filteredRes  []AIResource
	table        table.Model
	viewport     viewport.Model
	textInput    textinput.Model
	viewMode     ViewMode
	width        int
	height       int
	statusMsg    string
	statusExpiry time.Time
	currentDir   string
	selectedRes  *AIResource
	editMode     bool
	editField    int
	filterTag    string
	configFile   string
	showHelp     bool
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		showUsage()
		return
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}

	configFile := filepath.Join(homeDir, ".config/dwight/config.json")
	currentDir, _ := os.Getwd()

	// Ensure config directory exists
	os.MkdirAll(filepath.Dir(configFile), 0755)

	config := loadConfig(configFile)
	if config.TemplatesDir == "" {
		config.TemplatesDir = filepath.Join(homeDir, ".config/dwight/templates")
		os.MkdirAll(config.TemplatesDir, 0755)
	}

	if len(config.FileTypes) == 0 {
		config.FileTypes = []string{".md", ".txt", ".json", ".yaml", ".yml", ".py", ".js", ".ts"}
	}

	m := model{
		config:     config,
		configFile: configFile,
		currentDir: currentDir,
		width:      100,
		height:     24,
		viewMode:   ViewTable,
		editMode:   false,
		editField:  0,
		showHelp:   false,
	}

	// Initialize components
	m.textInput = textinput.New()
	m.textInput.CharLimit = 200

	// Initialize table
	columns := []table.Column{
		{Title: "Name", Width: 25},
		{Title: "Type", Width: 12},
		{Title: "Size", Width: 10},
		{Title: "Tags", Width: 20},
		{Title: "Modified", Width: 15},
		{Title: "Path", Width: 30},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m.table = t

	// Initialize viewport
	m.viewport = viewport.New(80, 20)

	// Load resources
	m.scanResources()
	m.updateTableData()

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func showUsage() {
	fmt.Println(`Dwight - AI Resource File Manager

USAGE:
    dwight [FLAGS]

FLAGS:
    -h, --help    Show this help message

FEATURES:
    • Scan and manage AI resource files (templates, prompts, contexts)
    • Create new templates from files or directories
    • Tag and categorize resources
    • Quick file content preview
    • Template instantiation

KEYBINDINGS:
    Navigation:
        ↑/↓         Navigate resources
        Enter/Space Launch/preview resource
        Tab         Switch between views
        
    Actions:
        n/a         Create new resource
        e           Edit resource metadata
        d           Delete resource
        t           Add/edit tags
        c           Create template from current file
        i           Show resource info
        
    Filters:
        f           Filter by tag
        /           Search resources
        r           Refresh scan
        
    Other:
        ?           Toggle help
        q           Quit`)
}

func loadConfig(configFile string) Config {
	var config Config
	data, err := os.ReadFile(configFile)
	if err != nil {
		return config
	}
	json.Unmarshal(data, &config)
	return config
}

func (m *model) saveConfig() {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(m.configFile, data, 0644)
}

func (m *model) scanResources() {
	m.resources = []AIResource{}

	// Scan current directory
	m.scanDirectory(m.currentDir)

	// Scan templates directory
	if m.config.TemplatesDir != "" {
		m.scanDirectory(m.config.TemplatesDir)
	}

	// Scan additional resource directories
	for _, dir := range m.config.ResourceDirs {
		m.scanDirectory(dir)
	}

	// Apply current filter
	m.applyFilter()
}

func (m *model) scanDirectory(dirPath string) {
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return
	}

	filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !m.isValidFileType(ext) {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		resource := AIResource{
			Name:       d.Name(),
			Path:       path,
			Type:       m.determineType(path),
			Size:       info.Size(),
			ModTime:    info.ModTime(),
			IsTemplate: strings.Contains(path, m.config.TemplatesDir),
		}

		// Load metadata if exists
		m.loadResourceMetadata(&resource)

		m.resources = append(m.resources, resource)
		return nil
	})
}

func (m *model) isValidFileType(ext string) bool {
	for _, validExt := range m.config.FileTypes {
		if ext == validExt {
			return true
		}
	}
	return false
}

func (m *model) determineType(path string) string {
	name := strings.ToLower(filepath.Base(path))

	if strings.Contains(name, "template") {
		return "template"
	} else if strings.Contains(name, "prompt") {
		return "prompt"
	} else if strings.Contains(name, "context") {
		return "context"
	} else if strings.Contains(name, "dataset") {
		return "dataset"
	} else if strings.Contains(path, m.config.TemplatesDir) {
		return "template"
	}

	return "resource"
}

func (m *model) loadResourceMetadata(resource *AIResource) {
	metaPath := resource.Path + ".meta"
	data, err := os.ReadFile(metaPath)
	if err != nil {
		return
	}

	var meta struct {
		Tags        []string `json:"tags"`
		Description string   `json:"description"`
	}

	if json.Unmarshal(data, &meta) == nil {
		resource.Tags = meta.Tags
		resource.Description = meta.Description
	}
}

func (m *model) saveResourceMetadata(resource *AIResource) {
	metaPath := resource.Path + ".meta"
	meta := struct {
		Tags        []string `json:"tags"`
		Description string   `json:"description"`
	}{
		Tags:        resource.Tags,
		Description: resource.Description,
	}

	data, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(metaPath, data, 0644)
}

func (m *model) applyFilter() {
	if m.filterTag == "" {
		m.filteredRes = m.resources
	} else {
		m.filteredRes = []AIResource{}
		for _, res := range m.resources {
			for _, tag := range res.Tags {
				if strings.Contains(strings.ToLower(tag), strings.ToLower(m.filterTag)) {
					m.filteredRes = append(m.filteredRes, res)
					break
				}
			}
		}
	}

	// Sort by modification time (newest first)
	sort.Slice(m.filteredRes, func(i, j int) bool {
		return m.filteredRes[i].ModTime.After(m.filteredRes[j].ModTime)
	})
}

func (m *model) updateTableData() {
	var rows []table.Row

	for _, res := range m.filteredRes {
		tags := strings.Join(res.Tags, ", ")
		if len(tags) > 18 {
			tags = tags[:18] + "..."
		}

		size := formatSize(res.Size)
		modTime := res.ModTime.Format("2006-01-02")

		// Truncate path for display
		displayPath := res.Path
		if len(displayPath) > 28 {
			displayPath = "..." + displayPath[len(displayPath)-25:]
		}

		rows = append(rows, table.Row{
			res.Name,
			res.Type,
			size,
			tags,
			modTime,
			displayPath,
		})
	}

	m.table.SetRows(rows)
}

func formatSize(size int64) string {
	if size < 1024 {
		return fmt.Sprintf("%dB", size)
	} else if size < 1024*1024 {
		return fmt.Sprintf("%.1fKB", float64(size)/1024)
	} else {
		return fmt.Sprintf("%.1fMB", float64(size)/(1024*1024))
	}
}

func (m *model) getSelectedResource() *AIResource {
	if len(m.filteredRes) == 0 {
		return nil
	}

	cursor := m.table.Cursor()
	if cursor >= 0 && cursor < len(m.filteredRes) {
		return &m.filteredRes[cursor]
	}
	return nil
}

func (m *model) createTemplate() {
	if m.selectedRes == nil {
		return
	}

	// Copy file to templates directory
	templateName := fmt.Sprintf("template_%s", m.selectedRes.Name)
	templatePath := filepath.Join(m.config.TemplatesDir, templateName)

	content, err := os.ReadFile(m.selectedRes.Path)
	if err != nil {
		return
	}

	err = os.WriteFile(templatePath, content, 0644)
	if err != nil {
		return
	}

	// Create template metadata
	template := AIResource{
		Name:        templateName,
		Path:        templatePath,
		Type:        "template",
		Tags:        []string{"template", "generated"},
		Description: fmt.Sprintf("Template created from %s", m.selectedRes.Name),
		IsTemplate:  true,
	}

	m.saveResourceMetadata(&template)
	m.scanResources()
	m.updateTableData()
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case statusMsg:
		m.statusMsg = msg.message
		m.statusExpiry = time.Now().Add(3 * time.Second)
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Update component sizes
		tableHeight := m.height - 8
		if tableHeight < 10 {
			tableHeight = 10
		}
		m.table.SetHeight(tableHeight)

		m.viewport.Width = m.width - 4
		m.viewport.Height = m.height - 8

		return m, nil

	case tea.KeyMsg:
		if m.editMode {
			return m.updateEdit(msg)
		}

		switch m.viewMode {
		case ViewTable:
			return m.updateTableView(msg)
		case ViewDetails:
			return m.updateDetails(msg)
		case ViewCreate:
			return m.updateCreate(msg)
		}
	}

	return m, nil
}

func (m model) updateEdit(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.editMode = false
		m.textInput.Blur()
		return m, nil
	case "enter":
		if m.selectedRes != nil {
			value := m.textInput.Value()
			switch m.editField {
			case 0: // Description
				m.selectedRes.Description = value
			case 1: // Tags
				m.selectedRes.Tags = strings.Split(value, ",")
				for i, tag := range m.selectedRes.Tags {
					m.selectedRes.Tags[i] = strings.TrimSpace(tag)
				}
			case 2: // Filter
				m.filterTag = value
				m.applyFilter()
				m.updateTableData()
			}
			if m.editField != 2 {
				m.saveResourceMetadata(m.selectedRes)
				m.scanResources()
				m.updateTableData()
			}
		}
		m.editMode = false
		m.textInput.Blur()
		return m, showStatus("✅ Updated")
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) updateTableView(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "?":
		m.showHelp = !m.showHelp
		return m, nil
	case "r":
		m.scanResources()
		m.updateTableData()
		return m, showStatus("🔄 Resources refreshed")
	case "tab":
		m.viewMode = ViewDetails
		m.selectedRes = m.getSelectedResource()
		if m.selectedRes != nil {
			content, err := os.ReadFile(m.selectedRes.Path)
			if err == nil {
				m.viewport.SetContent(string(content))
			}
		}
		return m, nil
	case "n", "a":
		m.viewMode = ViewCreate
		m.textInput.SetValue("")
		m.textInput.Focus()
		return m, nil
	case "e":
		m.selectedRes = m.getSelectedResource()
		if m.selectedRes != nil {
			m.editMode = true
			m.editField = 0
			m.textInput.SetValue(m.selectedRes.Description)
			m.textInput.Focus()
		}
		return m, nil
	case "t":
		m.selectedRes = m.getSelectedResource()
		if m.selectedRes != nil {
			m.editMode = true
			m.editField = 1
			m.textInput.SetValue(strings.Join(m.selectedRes.Tags, ", "))
			m.textInput.Focus()
		}
		return m, nil
	case "c":
		m.createTemplate()
		return m, showStatus("📝 Template created")
	case "f":
		m.editMode = true
		m.editField = 2 // Filter mode
		m.textInput.SetValue(m.filterTag)
		m.textInput.Focus()
		return m, nil
	case "i", "enter", " ":
		m.viewMode = ViewDetails
		m.selectedRes = m.getSelectedResource()
		if m.selectedRes != nil {
			content, err := os.ReadFile(m.selectedRes.Path)
			if err == nil {
				m.viewport.SetContent(string(content))
			}
		}
		return m, nil
	case "d":
		res := m.getSelectedResource()
		if res != nil {
			os.Remove(res.Path)
			os.Remove(res.Path + ".meta")
			m.scanResources()
			m.updateTableData()
			return m, showStatus("🗑️ Resource deleted")
		}
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) updateDetails(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc", "q":
		m.viewMode = ViewTable
		return m, nil
	case "tab":
		m.viewMode = ViewTable
		return m, nil
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

func (m model) updateCreate(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg.String() {
	case "esc":
		m.viewMode = ViewTable
		m.textInput.Blur()
		return m, nil
	case "enter":
		filename := m.textInput.Value()
		if filename != "" {
			filePath := filepath.Join(m.currentDir, filename)
			os.WriteFile(filePath, []byte("# New AI Resource\n\n"), 0644)
			m.scanResources()
			m.updateTableData()
			m.viewMode = ViewTable
			m.textInput.Blur()
			return m, showStatus("📄 Resource created")
		}
		return m, nil
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

func (m model) renderHeader() string {
	fullTitle := "🤖 Dwight - AI Resource Manager"
	
	// If terminal is wide enough, show full title
	if m.width >= len(fullTitle) {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
			Render(fullTitle)
	}
	
	// For medium width, show shorter version
	shortTitle := "🤖 Dwight - AI Manager"
	if m.width >= len(shortTitle) {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
			Render(shortTitle)
	}
	
	// For narrow terminals, show minimal version
	minimalTitle := "🤖 Dwight"
	if m.width >= len(minimalTitle) {
		return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
			Render(minimalTitle)
	}
	
	// For very narrow terminals, show just the icon
	return lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).
		Render("🤖")
}

func (m model) View() string {
	header := m.renderHeader()

	var statusMessage string
	if m.statusMsg != "" && time.Now().Before(m.statusExpiry) {
		color := lipgloss.Color("86")
		if strings.Contains(m.statusMsg, "❌") {
			color = lipgloss.Color("196")
		}
		statusStyle := lipgloss.NewStyle().Foreground(color)
		statusMessage = " > " + statusStyle.Render(m.statusMsg)
	}

	switch m.viewMode {
	case ViewTable:
		return m.viewTable(header, statusMessage)
	case ViewDetails:
		return m.viewDetails(header, statusMessage)
	case ViewCreate:
		return m.viewCreate(header, statusMessage)
	}

	return ""
}

func (m model) viewTable(header, statusMessage string) string {
	if m.showHelp {
		return m.viewHelp(header)
	}

	if len(m.filteredRes) == 0 {
		content := "\nNo AI resources found.\n\nPress 'n' to create a new resource or 'r' to refresh."
		footer := "n: new resource • r: refresh • q: quit • ?: help"
		return fmt.Sprintf("%s\n%s\n\n%s", header, content, footer)
	}

	// Show current directory and filter info
	info := fmt.Sprintf("📁 %s", m.currentDir)
	if m.filterTag != "" {
		info += fmt.Sprintf(" | 🏷️  Filter: %s", m.filterTag)
	}
	info += fmt.Sprintf(" | 📊 %d resources", len(m.filteredRes))

	var footer string
	if m.editMode {
		fieldNames := []string{"Description", "Tags", "Filter"}
		fieldName := fieldNames[m.editField]
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		footer = fmt.Sprintf("Editing %s: %s | %s: save • %s: cancel",
			fieldName, m.textInput.View(), keyStyle.Render("enter"), keyStyle.Render("esc"))
	} else {
		keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
		actionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("86"))
		bulletStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

		footer = fmt.Sprintf("%s: %s %s %s: %s %s %s: %s %s %s: %s %s %s: %s\n%s: %s %s %s: %s %s %s: %s %s %s: %s %s %s: %s %s %s: %s%s %s %s: %s\n",
			keyStyle.Render("↑↓"), actionStyle.Render("navigate"), bulletStyle.Render("•"),
			keyStyle.Render("enter"), actionStyle.Render("view"), bulletStyle.Render("•"),
			keyStyle.Render("tab"), actionStyle.Render("details"), bulletStyle.Render("•"),
			keyStyle.Render("n"), actionStyle.Render("new"), bulletStyle.Render("•"),
			keyStyle.Render("e"), actionStyle.Render("edit"),
			keyStyle.Render("t"), actionStyle.Render("tags"), bulletStyle.Render("•"),
			keyStyle.Render("c"), actionStyle.Render("template"), bulletStyle.Render("•"),
			keyStyle.Render("f"), actionStyle.Render("filter"), bulletStyle.Render("•"),
			keyStyle.Render("d"), actionStyle.Render("delete"), bulletStyle.Render("•"),
			keyStyle.Render("r"), actionStyle.Render("refresh"), bulletStyle.Render("•"),
			keyStyle.Render("?"), actionStyle.Render("help"), bulletStyle.Render("•"),
			keyStyle.Render("q"), actionStyle.Render("quit"),
			statusMessage)
	}

	return fmt.Sprintf("%s\n\n%s\n\n%s\n\n%s", header, info, m.table.View(), footer)
}

func (m model) viewDetails(header, statusMessage string) string {
	if m.selectedRes == nil {
		return fmt.Sprintf("%s\n\nNo resource selected\n\nPress 'esc' to return", header)
	}

	details := fmt.Sprintf("📄 %s\n", m.selectedRes.Name)
	details += fmt.Sprintf("📁 %s\n", m.selectedRes.Path)
	details += fmt.Sprintf("🏷️  %s\n", m.selectedRes.Type)
	details += fmt.Sprintf("📊 %s\n", formatSize(m.selectedRes.Size))
	details += fmt.Sprintf("🕒 %s\n", m.selectedRes.ModTime.Format("2006-01-02 15:04:05"))

	if len(m.selectedRes.Tags) > 0 {
		details += fmt.Sprintf("🏷️  Tags: %s\n", strings.Join(m.selectedRes.Tags, ", "))
	}

	if m.selectedRes.Description != "" {
		details += fmt.Sprintf("📝 %s\n", m.selectedRes.Description)
	}

	details += "\n" + strings.Repeat("─", 50) + "\n"

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	footer := fmt.Sprintf("%s: back • %s: scroll • %s: quit",
		keyStyle.Render("esc"), keyStyle.Render("↑↓"), keyStyle.Render("q"))

	return fmt.Sprintf("%s\n\n%s\n%s\n\n%s\n%s", header, details, m.viewport.View(), footer, statusMessage)
}

func (m model) viewCreate(header, statusMessage string) string {
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))

	content := fmt.Sprintf("Create new resource in: %s\n\nFilename: %s\n\n%s: create • %s: cancel",
		m.currentDir, m.textInput.View(), keyStyle.Render("enter"), keyStyle.Render("esc"))

	return fmt.Sprintf("%s\n\n%s\n%s", header, content, statusMessage)
}

func (m model) viewHelp(header string) string {
	help := `
KEYBINDINGS:

Navigation:
    ↑/↓         Navigate resources
    Enter/Space View resource details
    Tab         Switch to details view
    
Actions:
    n/a         Create new resource
    e           Edit resource description
    t           Edit resource tags
    d           Delete resource
    c           Create template from resource
    
Filters & Search:
    f           Filter by tag
    r           Refresh resource scan
    
Other:
    ?           Toggle this help
    q           Quit application

RESOURCE TYPES:
    template    Template files for reuse
    prompt      AI prompt files
    context     Context/background files
    dataset     Data files
    resource    General resource files

CONFIGURATION:
    Config file: ~/.config/dwight/config.json
    Templates:   ~/.config/dwight/templates/
    
    Supported file types: .md, .txt, .json, .yaml, .yml, .py, .js, .ts
`

	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	footer := fmt.Sprintf("%s: close help", keyStyle.Render("?"))

	return fmt.Sprintf("%s\n%s\n\n%s", header, help, footer)
}
