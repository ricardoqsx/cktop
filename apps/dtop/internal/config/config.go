package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type MemoryMode string

const (
	MemoryUsage   MemoryMode = "usage"
	MemoryPercent MemoryMode = "percent"
	MemoryBoth    MemoryMode = "both"
)

type Config struct {
	Display            Display
	ComposeProjects    []ComposeProject
	ComposeDiagnostics []string
}

type Display struct {
	MemoryMode MemoryMode
}

// ComposeProject is a validated, read-only registration from dtop.conf.
type ComposeProject struct {
	Name         string
	WorkingDir   string
	Files        []string
	MissingFiles []string
}

type Paths struct {
	System string
	User   string
}

func Default() Config {
	return Config{Display: Display{MemoryMode: MemoryBoth}}
}

func Load() (Config, error) {
	return LoadPaths(defaultPaths())
}

func LoadPaths(paths Paths) (Config, error) {
	config := Default()
	for _, path := range []string{paths.System, paths.User} {
		if path == "" {
			continue
		}
		if err := mergeFile(&config, path); err != nil {
			return Config{}, err
		}
	}

	return config, nil
}

func defaultPaths() Paths {
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		if home, err := os.UserHomeDir(); err == nil {
			configHome = filepath.Join(home, ".config")
		}
	}

	userPath := ""
	if configHome != "" {
		userPath = filepath.Join(configHome, "dtop", "dtop.conf")
	}

	return Paths{
		System: "/etc/dtop/dtop.conf",
		User:   userPath,
	}
}

func mergeFile(config *Config, path string) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open config %s: %w", path, err)
	}
	defer file.Close()

	section := ""
	var compose *composeSection
	finishCompose := func() {
		if compose == nil {
			return
		}
		project, diagnostic := validateComposeProject(*compose)
		if diagnostic != "" {
			config.ComposeDiagnostics = append(config.ComposeDiagnostics, diagnostic)
		} else {
			replaceComposeProject(&config.ComposeProjects, project)
		}
		compose = nil
	}
	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			finishCompose()
			section = strings.TrimSpace(line[1 : len(line)-1])
			if strings.EqualFold(section, "display") {
				section = "display"
				continue
			}
			name, ok := composeSectionName(section)
			if !ok {
				if strings.HasPrefix(strings.ToLower(section), "compose") {
					compose = &composeSection{path: path, line: lineNumber, invalid: fmt.Sprintf("config %s:%d: malformed Compose registration header", path, lineNumber)}
					continue
				}
				return configError(path, lineNumber, "unsupported section %q", section)
			}
			compose = &composeSection{name: name, path: path, line: lineNumber}
			continue
		}

		key, value, found := strings.Cut(line, "=")
		if !found {
			return configError(path, lineNumber, "expected key = value")
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if compose != nil {
			switch key {
			case "working_dir":
				compose.workingDir = value
			case "files":
				compose.files = value
			default:
				compose.invalid = configError(path, lineNumber, "unsupported key %q in Compose registration %q", key, compose.name).Error()
			}
			continue
		}
		if section != "display" || key != "memory_mode" {
			return configError(path, lineNumber, "unsupported key %q in section %q", key, section)
		}

		mode := MemoryMode(strings.ToLower(value))
		if !mode.Valid() {
			return configError(path, lineNumber, "memory_mode must be usage, percent, or both")
		}
		config.Display.MemoryMode = mode
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	finishCompose()

	return nil
}

type composeSection struct {
	name       string
	workingDir string
	files      string
	path       string
	line       int
	invalid    string
}

func composeSectionName(section string) (string, bool) {
	const prefix = "compose \""
	if !strings.HasPrefix(section, prefix) || !strings.HasSuffix(section, "\"") {
		return "", false
	}
	name := strings.TrimSpace(section[len(prefix) : len(section)-1])
	return name, name != ""
}

func validateComposeProject(section composeSection) (ComposeProject, string) {
	if section.invalid != "" {
		return ComposeProject{}, section.invalid
	}
	prefix := fmt.Sprintf("config %s:%d: Compose registration %q", section.path, section.line, section.name)
	if section.name == "" || strings.TrimSpace(section.workingDir) == "" || strings.TrimSpace(section.files) == "" {
		return ComposeProject{}, prefix + " requires a nonempty name, working_dir, and files"
	}
	workingDir, err := filepath.Abs(section.workingDir)
	if err != nil {
		return ComposeProject{}, prefix + ": resolve working_dir: " + err.Error()
	}
	info, err := os.Stat(workingDir)
	if err != nil || !info.IsDir() {
		return ComposeProject{}, prefix + " has an unavailable working_dir"
	}

	project := ComposeProject{Name: section.name, WorkingDir: filepath.Clean(workingDir)}
	for _, file := range strings.Split(section.files, ",") {
		file = strings.TrimSpace(file)
		if file == "" {
			return ComposeProject{}, prefix + " has an empty files entry"
		}
		if !filepath.IsAbs(file) {
			file = filepath.Join(project.WorkingDir, file)
		}
		file = filepath.Clean(file)
		project.Files = append(project.Files, file)
		info, err := os.Stat(file)
		if err != nil || info.IsDir() {
			project.MissingFiles = append(project.MissingFiles, file)
		}
	}
	return project, ""
}

func replaceComposeProject(projects *[]ComposeProject, project ComposeProject) {
	for index, existing := range *projects {
		if existing.Name == project.Name {
			(*projects)[index] = project
			return
		}
	}
	*projects = append(*projects, project)
}

func (m MemoryMode) Valid() bool {
	return m == MemoryUsage || m == MemoryPercent || m == MemoryBoth
}

func configError(path string, line int, format string, args ...any) error {
	return fmt.Errorf("config %s:%d: %s", path, line, fmt.Sprintf(format, args...))
}
