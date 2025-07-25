package main

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

func (m *model) scanResources() {
	m.resources = []AIResource{}

	// ONLY scan current directory recursively - no global templates
	scanDir := m.currentDir
	m.scanDirectory(scanDir)

	// Reconcile metadata with actual filesystem
	m.reconcileMetadata()

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

		if d.Name() == ProjectMetaFile {
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

		m.loadResourceMetadata(&resource)

		m.resources = append(m.resources, resource)
		return nil
	})
}

func (m *model) scanGlobalResources() {
	m.globalRes = []AIResource{}

	if m.config.TemplatesDir != "" {
		dwightDir := filepath.Dir(m.config.TemplatesDir)
		m.scanGlobalDirectory(dwightDir)
	}

	sort.Slice(m.globalRes, func(i, j int) bool {
		return m.globalRes[i].ModTime.After(m.globalRes[j].ModTime)
	})
}

func (m *model) scanGlobalDirectory(dirPath string) {
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

		if d.Name() == "config.json" {
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

		isTemplate := strings.Contains(path, filepath.Join(dirPath, "templates"))

		var tags []string
		var description string

		if isTemplate {
			tags = []string{"global", "template"}
			description = "Global template resource"
		} else {
			tags = []string{"global"}
			description = "Global resource"
		}

		resource := AIResource{
			Name:        d.Name(),
			Path:        path,
			Type:        m.determineType(path),
			Size:        info.Size(),
			ModTime:     info.ModTime(),
			IsTemplate:  isTemplate,
			Tags:        tags,
			Description: description,
		}

		m.globalRes = append(m.globalRes, resource)
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

	if strings.Contains(name, "config") {
		return "config"
	} else if strings.Contains(name, "setting") {
		return "settings"
	} else if strings.Contains(name, "template") {
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
	if m.projectMeta == nil {
		return
	}

	relPath, err := filepath.Rel(m.projectRoot, resource.Path)
	if err != nil {
		relPath = resource.Name
	}

	if meta, exists := m.projectMeta.Resources[relPath]; exists {
		resource.Tags = meta.Tags
		resource.Description = meta.Description
		if meta.Type != "" {
			resource.Type = meta.Type
		}
	}
}

func (m *model) saveResourceMetadata(resource *AIResource) {
	if m.projectMeta == nil {
		return
	}

	relPath, err := filepath.Rel(m.projectRoot, resource.Path)
	if err != nil {
		relPath = resource.Name
	}

	m.projectMeta.Resources[relPath] = ResourceMetadata{
		Tags:         resource.Tags,
		Description:  resource.Description,
		Type:         resource.Type,
		LastModified: time.Now(),
	}

	m.saveProjectMetadata()
}

func (m *model) reconcileMetadata() {
	if m.projectMeta == nil {
		return
	}

	// Create a map of currently scanned files for quick lookup
	scannedFiles := make(map[string]bool)
	for _, resource := range m.resources {
		if m.projectRoot != "" {
			if relPath, err := filepath.Rel(m.projectRoot, resource.Path); err == nil {
				scannedFiles[relPath] = true
			}
		}
	}

	// Add metadata for new files that don't have it yet
	for _, resource := range m.resources {
		var relPath string
		if m.projectRoot != "" {
			if rp, err := filepath.Rel(m.projectRoot, resource.Path); err == nil {
				relPath = rp
			} else {
				relPath = resource.Name
			}
		} else {
			relPath = resource.Name
		}

		if _, exists := m.projectMeta.Resources[relPath]; !exists {
			// Add new file with default metadata
			m.projectMeta.Resources[relPath] = ResourceMetadata{
				Tags:         []string{},
				Description:  "",
				Type:         resource.Type,
				LastModified: time.Now(),
			}
		}
	}

	// Remove orphaned metadata for files that no longer exist
	for relPath := range m.projectMeta.Resources {
		if !scannedFiles[relPath] {
			delete(m.projectMeta.Resources, relPath)
		}
	}

	// Save updated metadata
	m.saveProjectMetadata()
}

func (m *model) applyFilter() {
	if m.filterTag == "" {
		m.filteredRes = m.resources
	} else {
		m.filteredRes = []AIResource{}
		searchTerm := strings.ToLower(m.filterTag)

		for _, res := range m.resources {
			matched := false

			if strings.Contains(strings.ToLower(res.Name), searchTerm) {
				matched = true
			}

			if !matched {
				for _, tag := range res.Tags {
					if strings.Contains(strings.ToLower(tag), searchTerm) {
						matched = true
						break
					}
				}
			}

			if !matched && strings.Contains(strings.ToLower(res.Description), searchTerm) {
				matched = true
			}

			if !matched && strings.Contains(strings.ToLower(res.Type), searchTerm) {
				matched = true
			}

			if matched {
				m.filteredRes = append(m.filteredRes, res)
			}
		}
	}

	// Apply sorting
	sort.Slice(m.filteredRes, func(i, j int) bool {
		var result bool
		switch m.sortBy {
		case "name":
			result = strings.ToLower(m.filteredRes[i].Name) < strings.ToLower(m.filteredRes[j].Name)
		case "type":
			result = strings.ToLower(m.filteredRes[i].Type) < strings.ToLower(m.filteredRes[j].Type)
		case "size":
			result = m.filteredRes[i].Size < m.filteredRes[j].Size
		case "modified":
			result = m.filteredRes[i].ModTime.Before(m.filteredRes[j].ModTime)
		default: // Default to modified time (newest first)
			result = m.filteredRes[i].ModTime.After(m.filteredRes[j].ModTime)
		}

		if m.sortDesc {
			return !result
		}
		return result
	})
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

func findProjectRoot(startDir string) string {
	dir := startDir
	for {
		indicators := []string{".git", "package.json", "pyproject.toml", "Cargo.toml", "go.mod", ".project"}
		for _, indicator := range indicators {
			if _, err := os.Stat(filepath.Join(dir, indicator)); err == nil {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return startDir
		}
		dir = parent
	}
}

func (m *model) loadProjectMetadata() {
	if m.projectRoot == "" {
		return
	}

	metaPath := filepath.Join(m.projectRoot, ProjectMetaFile)
	data, err := os.ReadFile(metaPath)
	if err != nil {
		m.createInitialSetup()
		return
	}

	var meta ProjectMetadata
	if json.Unmarshal(data, &meta) == nil {
		m.projectMeta = &meta
	}
}

func (m *model) createInitialSetup() {
	m.projectMeta = &ProjectMetadata{
		ProjectName:   filepath.Base(m.projectRoot),
		Created:       time.Now(),
		Resources:     make(map[string]ResourceMetadata),
		TemplatesUsed: []string{},
		Settings: ProjectSettings{
			DefaultModel: "claude-sonnet-4",
			AutoScan:     true,
		},
	}
	m.saveProjectMetadata()
}

func (m *model) saveProjectMetadata() {
	if m.projectMeta == nil || m.projectRoot == "" {
		return
	}

	metaPath := filepath.Join(m.projectRoot, ProjectMetaFile)
	data, err := json.MarshalIndent(m.projectMeta, "", "  ")
	if err != nil {
		return
	}

	os.WriteFile(metaPath, data, 0644)
}

// pushResourceToGlobal copies a local resource to the global templates directory
func (m *model) pushResourceToGlobal(resource *AIResource) error {
	if resource == nil {
		return fmt.Errorf("no resource selected")
	}

	// Determine destination path in global directory
	globalDir := filepath.Dir(m.config.TemplatesDir)
	templateSubdir := "templates"
	
	// Create destination directory structure
	destDir := filepath.Join(globalDir, templateSubdir)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create global directory: %w", err)
	}

	// Create destination file path
	destPath := filepath.Join(destDir, filepath.Base(resource.Path))
	
	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("file already exists in global directory")
	}

	// Copy file content
	content, err := os.ReadFile(resource.Path)
	if err != nil {
		return fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write to global directory: %w", err)
	}

	return nil
}

// pullResourceFromGlobal copies a global resource to the local project templates directory
func (m *model) pullResourceFromGlobal(resource *AIResource) error {
	if resource == nil {
		return fmt.Errorf("no resource selected")
	}

	// Determine destination path in project
	var destDir string
	if m.projectRoot != "" {
		// Create project templates directory if it doesn't exist
		destDir = filepath.Join(m.projectRoot, "templates")
	} else {
		// Use current directory if no project root
		destDir = m.currentDir
	}

	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create project templates directory: %w", err)
	}

	// Create destination file path
	destPath := filepath.Join(destDir, filepath.Base(resource.Path))
	
	// Check if file already exists
	if _, err := os.Stat(destPath); err == nil {
		return fmt.Errorf("file already exists in project")
	}

	// Copy file content
	content, err := os.ReadFile(resource.Path)
	if err != nil {
		return fmt.Errorf("failed to read global file: %w", err)
	}

	if err := os.WriteFile(destPath, content, 0644); err != nil {
		return fmt.Errorf("failed to write to project directory: %w", err)
	}

	// Add metadata to project if we have project metadata
	if m.projectMeta != nil {
		relPath, err := filepath.Rel(m.projectRoot, destPath)
		if err != nil {
			relPath = filepath.Base(destPath)
		}

		m.projectMeta.Resources[relPath] = ResourceMetadata{
			Tags:         append(resource.Tags, "pulled-from-global"),
			Description:  resource.Description + " (pulled from global)",
			Type:         resource.Type,
			LastModified: time.Now(),
		}
		m.saveProjectMetadata()
	}

	return nil
}
