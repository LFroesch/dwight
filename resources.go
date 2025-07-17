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

	// ONLY scan local directory - no global templates
	scanDir := m.currentDir
	if m.projectRoot != "" {
		scanDir = m.projectRoot
	}
	m.scanDirectory(scanDir)

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

	sort.Slice(m.filteredRes, func(i, j int) bool {
		return m.filteredRes[i].ModTime.After(m.filteredRes[j].ModTime)
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

func (m *model) createTemplate() {
	if m.selectedRes == nil {
		return
	}

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

	if m.projectMeta != nil {
		templateExists := false
		for _, t := range m.projectMeta.TemplatesUsed {
			if t == templateName {
				templateExists = true
				break
			}
		}
		if !templateExists {
			m.projectMeta.TemplatesUsed = append(m.projectMeta.TemplatesUsed, templateName)
			m.saveProjectMetadata()
		}
	}

	m.scanResources()
	m.updateTableData()
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

	templatePath := filepath.Join(m.projectRoot, "template.md")
	if _, err := os.Stat(templatePath); os.IsNotExist(err) {
		m.createDefaultTemplate(templatePath)
	}
}

func (m *model) createDefaultTemplate(templatePath string) {
	defaultTemplate := `# AI Assistant Instructions Template

## Project Context
This is a template for providing context and instructions to AI assistants working on this project.

## Project Overview
- **Project Name**: ` + filepath.Base(m.projectRoot) + `
- **Created**: ` + time.Now().Format("2006-01-02") + `
- **Purpose**: [Describe the purpose of your project]

## Key Information
- **Technology Stack**: [List technologies used]
- **Architecture**: [Describe system architecture]
- **Coding Standards**: [List coding conventions]

## Instructions for AI Assistants
1. Follow existing code patterns and conventions
2. Maintain consistency with project structure
3. Write clear, documented code
4. Test changes thoroughly

## Important Files and Directories
- ` + "`" + `/src/` + "`" + ` - [Describe source directory]
- ` + "`" + `/docs/` + "`" + ` - [Describe documentation]
- ` + "`" + `/tests/` + "`" + ` - [Describe test structure]

## Notes
[Add any specific notes or requirements for AI assistants]

---
*This template was generated by dwight. Edit it to better suit your project needs.*
`

	err := os.WriteFile(templatePath, []byte(defaultTemplate), 0644)
	if err != nil {
		fmt.Printf("Failed to create template.md: %v", err)
	}
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