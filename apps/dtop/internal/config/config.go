package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type MemoryMode string

const (
	MemoryUsage   MemoryMode = "usage"
	MemoryPercent MemoryMode = "percent"
	MemoryBoth    MemoryMode = "both"
)

type Config struct {
	Display            Display
	Updates            Updates
	ComposeProjects    []ComposeProject
	ComposeDiagnostics []string
}

type Display struct {
	MemoryMode  MemoryMode
	AccentColor string
	FocusColor  string
}

type Updates struct {
	Enabled     bool
	Scope       string
	Interval    time.Duration
	Concurrency int
}

// ComposeProject is a validated, read-only registration from dtop.conf.
type ComposeProject struct {
	Name         string
	WorkingDir   string
	Files        []string
	MissingFiles []string
}

type Paths struct {
	System      string
	User        string
	Environment string
	Explicit    string
}

func Default() Config {
	return Config{Display: Display{MemoryMode: MemoryBoth, AccentColor: "63", FocusColor: "15"}, Updates: Updates{Enabled: true, Scope: "running", Interval: 15 * time.Minute, Concurrency: 4}}
}

func Load() (Config, error) {
	return LoadWithPath("")
}

func LoadWithPath(path string) (Config, error) {
	paths := defaultPaths()
	paths.Environment = os.Getenv("DTOP_CONFIG")
	paths.Explicit = path
	return LoadPaths(paths)
}

func LoadPaths(paths Paths) (Config, error) {
	config := Default()
	sources := []struct {
		name     string
		path     string
		required bool
	}{
		{name: "system", path: paths.System},
		{name: "user", path: paths.User},
		{name: "DTOP_CONFIG", path: paths.Environment, required: paths.Environment != ""},
		{name: "--config", path: paths.Explicit, required: paths.Explicit != ""},
	}
	for _, source := range sources {
		if source.path == "" {
			continue
		}
		if err := mergeFile(&config, source.path, source.required); err != nil {
			if source.required && errors.Is(err, os.ErrNotExist) {
				return Config{}, fmt.Errorf("%s file %s does not exist", source.name, source.path)
			}
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

func mergeFile(config *Config, path string, required bool) error {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) && !required {
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
			if strings.EqualFold(section, "display") || strings.EqualFold(section, "updates") {
				section = strings.ToLower(section)
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
		if section != "display" && section != "updates" {
			return configError(path, lineNumber, "unsupported key %q in section %q", key, section)
		}
		if section == "updates" {
			switch key {
			case "enabled":
				value, err := strconv.ParseBool(value)
				if err != nil {
					return configError(path, lineNumber, "enabled must be true or false")
				}
				config.Updates.Enabled = value
			case "scope":
				if strings.ToLower(value) != "running" {
					return configError(path, lineNumber, "scope must be running")
				}
				config.Updates.Scope = "running"
			case "interval":
				duration, err := time.ParseDuration(value)
				if err != nil || duration < time.Minute {
					return configError(path, lineNumber, "interval must be at least 1m")
				}
				config.Updates.Interval = duration
			case "concurrency":
				concurrency, err := strconv.Atoi(value)
				if err != nil || concurrency < 1 || concurrency > 16 {
					return configError(path, lineNumber, "concurrency must be from 1 through 16")
				}
				config.Updates.Concurrency = concurrency
			default:
				return configError(path, lineNumber, "unsupported key %q in section %q", key, section)
			}
			continue
		}
		switch key {
		case "memory_mode":
			mode := MemoryMode(strings.ToLower(value))
			if !mode.Valid() {
				return configError(path, lineNumber, "memory_mode must be usage, percent, or both")
			}
			config.Display.MemoryMode = mode
		case "accent_color":
			if !validANSIColor(value) {
				return configError(path, lineNumber, "accent_color must be an ANSI color from 0 through 255")
			}
			config.Display.AccentColor = value
		case "focus_color":
			if !validANSIColor(value) {
				return configError(path, lineNumber, "focus_color must be an ANSI color from 0 through 255")
			}
			config.Display.FocusColor = value
		default:
			return configError(path, lineNumber, "unsupported key %q in section %q", key, section)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	finishCompose()

	return nil
}

func validANSIColor(value string) bool {
	color, err := strconv.Atoi(value)
	return err == nil && color >= 0 && color <= 255
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
