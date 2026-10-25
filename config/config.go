package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/goccy/go-yaml"
	"hermannm.dev/errclose"
	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/internal/sliceutil"
)

type Config struct {
	DefaultSTC *STC      `yaml:"defaultSTC,omitempty"`
	Projects   []Project `yaml:"projects"`
	// Omitted from YAML, since we set this ourselves.
	//exhaustruct:optional
	Path Path `yaml:"-"`
	// We include the user's home directory in the config struct after parsing, so that we don't
	// have to call [os.UserHomeDir] again later.
	//exhaustruct:optional
	Homedir string `yaml:"-"`
}

type Project struct {
	Path string `yaml:"path"`
	STC  *STC   `yaml:"STC,omitempty"`
}

// STC = Standard Template Construct = AI model.
type STC struct {
	Model string   `yaml:"model"`
	Args  []string `yaml:"args,omitempty"`
}

func Load() (cfg Config, returnedErr error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, wrap.Error(err, "could not get user home dir")
	}
	configPath := getConfigPath(homedir)

	configFile, err := os.Open(configPath.Absolute)
	if err != nil {
		return Config{}, wrap.ErrorWithAttrs(
			err,
			"failed to open config file",
			"path", configPath.Short,
		)
	}
	defer errclose.Closef(configFile, &returnedErr, "config file %s", configPath.Short)

	if err := yaml.NewDecoder(configFile).Decode(&cfg); err != nil {
		return Config{}, wrap.ErrorWithAttrs(
			err,
			"failed to parse config file as YAML",
			"path", configPath.Short,
		)
	}
	cfg.Path = configPath
	cfg.Homedir = homedir

	// Expand uses of ~ in project paths
	for i, project := range cfg.Projects {
		cfg.Projects[i].Path = ExpandHomedir(project.Path, homedir)

		if project.STC == nil && cfg.DefaultSTC == nil {
			return Config{}, wrap.ErrorWithAttrs(
				err,
				"either `defaultSTC` or `project.STC` must be set in config",
				"path", configPath.Short,
			)
		}
	}

	return cfg, nil
}

func (cfg Config) GetCurrentProject() (Project, error) {
	path, err := os.Getwd()
	if err != nil {
		return Project{}, wrap.Error(err, "failed to get current directory to determine project")
	}

	project, ok := sliceutil.Find(
		cfg.Projects,
		func(p Project) bool {
			return strings.HasPrefix(path, p.Path)
		},
	)
	if !ok {
		return Project{}, fmt.Errorf(
			"'%s' is not a configured project path in %s",
			CollapseHomedir(path, cfg.Homedir), cfg.Path.Short,
		)
	}

	if project.STC == nil {
		project.STC = cfg.DefaultSTC
	}

	return project, nil
}

func Initialize() (returnErr error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return wrap.Error(err, "could not get user home dir")
	}
	configPath := getConfigPath(homedir)

	if fileExists(configPath.Absolute) {
		slog.Info(fmt.Sprintf("%s already exists, skipping initialization", configPath.Short))
		return nil
	}

	configDir, _ := filepath.Split(configPath.Absolute)
	if err := os.MkdirAll(configDir, configDirPermissions); err != nil {
		return wrap.ErrorWithAttrs(
			err,
			"failed to create directory for Omnissiah config",
			"path", configPath.Short,
		)
	}

	configFile, err := os.Create(configPath.Absolute)
	if err != nil {
		return wrap.ErrorWithAttrs(
			err,
			"failed to create Omnissiah config file",
			"path", configPath.Short,
		)
	}
	defer errclose.Closef(configFile, &returnErr, "config file %s", configPath.Short)

	encoder := yaml.NewEncoder(configFile)
	defer errclose.Close(encoder, &returnErr, "YAML encoder")

	startingConfig := Config{
		DefaultSTC: new(
			STC{
				Model: "claude",
				Args:  []string{"--model=opus"},
			},
		),
		Projects: []Project{{
			Path: filepath.Join("~", "example"),
			STC:  nil,
		}},
	}

	if err := encoder.Encode(startingConfig); err != nil {
		return wrap.Error(err, "failed to encode config YAML")
	}

	slog.Info("Initialized Omnissiah config", "path", configPath.Short, "config", startingConfig)
	return nil
}

// File permissions used by most under directories under `~/.config`.
const configDirPermissions = 0755

type Path struct {
	Absolute string
	Short    string
}

func getConfigPath(homedir string) Path {
	// Honor XDG_CONFIG_HOME env variable for config file directory
	// https://specifications.freedesktop.org/basedir/latest/
	configHome := os.Getenv("XDG_CONFIG_HOME")
	if configHome == "" {
		// Default to ~/.config
		configHome = filepath.Join("~", ".config")
	} else {
		// If XDG_CONFIG_HOME is set, collapse home dir in path to ~, so we can return both variants
		// below
		configHome = CollapseHomedir(configHome, homedir)
	}

	path := filepath.Join(configHome, "omnissiah", "config.yml")

	return Path{
		Absolute: ExpandHomedir(path, homedir),
		Short:    path,
	}
}

// Known values for [STC.Model].
//
// See "Available agents" at https://docs.docker.com/reference/cli/sbx/run/
const (
	STCClaude  = "claude"
	STCCodex   = "codex"
	STCCopilot = "copilot"
	STCGemini  = "gemini"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func ExpandHomedir(path string, homedir string) (expandedPath string) {
	return strings.Replace(path, "~", homedir, 1)
}

func CollapseHomedir(path string, homedir string) (collapsedPath string) {
	return strings.Replace(path, homedir, "~", 1)
}
