package project

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cwel/kmux/internal/config"
)

// Project represents a discovered directory/repository.
type Project struct {
	Name string // derived from directory name
	Path string // full path to the project
}

// Scanner discovers projects from configured directories.
type Scanner struct {
	dirs     []string
	maxDepth int
	ignore   []string
	gitOnly  bool
}

// NewScanner creates a scanner from config.
func NewScanner(cfg *config.Config) *Scanner {
	dirs := make([]string, len(cfg.Projects.Directories))
	for i, d := range cfg.Projects.Directories {
		dirs[i] = config.ExpandPath(d)
	}
	return &Scanner{
		dirs:     dirs,
		maxDepth: cfg.Projects.MaxDepth,
		ignore:   cfg.Projects.Ignore,
		gitOnly:  cfg.Projects.GitOnly,
	}
}

// Scan finds all projects in configured directories.
func (s *Scanner) Scan() []Project {
	seen := make(map[string]bool)
	var projects []Project

	for _, dir := range s.dirs {
		s.scanDir(dir, 0, &projects, seen)
	}

	// Sort by name
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Name < projects[j].Name
	})

	return projects
}

// isIgnored checks if a path matches any ignore pattern.
func (s *Scanner) isIgnored(path string) bool {
	name := filepath.Base(path)
	for _, pattern := range s.ignore {
		// Check against full path
		if matched, _ := filepath.Match(pattern, path); matched {
			return true
		}
		// Check against just the name
		if matched, _ := filepath.Match(pattern, name); matched {
			return true
		}
		// Check if pattern is a substring of path (for absolute paths in ignore)
		expanded := config.ExpandPath(pattern)
		if expanded == path || strings.HasPrefix(path, expanded+"/") {
			return true
		}
	}
	return false
}

func (s *Scanner) scanDir(dir string, depth int, projects *[]Project, seen map[string]bool) {
	if depth > s.maxDepth {
		return
	}

	// Check if ignored
	if s.isIgnored(dir) {
		return
	}

	name := filepath.Base(dir)

	// Check if this directory is a git repo
	gitDir := filepath.Join(dir, ".git")
	isGitRepo := false
	if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
		isGitRepo = true
	}

	// Add as project if: it's a git repo, OR git_only is false and we're at depth > 0
	if isGitRepo || (!s.gitOnly && depth > 0) {
		if !seen[name] {
			seen[name] = true
			*projects = append(*projects, Project{
				Name: name,
				Path: dir,
			})
		}
		if isGitRepo {
			return // Don't recurse into git repos
		}
	}

	// Recurse into subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip hidden directories
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		s.scanDir(filepath.Join(dir, entry.Name()), depth+1, projects, seen)
	}
}

// FilterExisting removes projects that already have sessions.
func FilterExisting(projects []Project, sessionNames map[string]bool) []Project {
	var filtered []Project
	for _, p := range projects {
		if !sessionNames[p.Name] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}
