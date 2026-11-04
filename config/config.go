package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"hermannm.dev/errclose"
	"hermannm.dev/wrap"

	"hermannm.dev/omnissiah/internal/sliceutil"
)

type Config struct {
	DefaultSTC *STC `yaml:"defaultSTC,omitempty"`
	// sbx kits to apply to all Omnissiah prayers:
	// https://docs.docker.com/ai/sandboxes/customize/use-kits/
	DefaultKits []Kit     `yaml:"defaultKits,omitempty"`
	Projects    []Project `yaml:"projects"`
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
	// sbx kits to apply to Omnissiah prayers in this project
	Kits []Kit `yaml:"kits,omitempty"`
}

type Kit struct {
	// From sbx docs: "directory, ZIP, git, or OCI"
	Path string `yaml:"path"`
	// Set with [Kit.setAbsolutePath].
	//exhaustruct:optional
	AbsolutePath string `yaml:"-"`
	// Arguments to pass to the kit (Docker build args).
	Args []string `yaml:"args,omitempty"`
}

// Name returns the sbx "name" of the kit, which is the last path component of the kit's path.
// Used to set kit args for specific kits.
//
// Not really well documented, but hinted at here: "A workload's agent name comes from the last
// component of its repository name"
// https://docs.docker.com/ai/sandboxes/customize/use-kits/#published-v3-workloads
func (kit Kit) Name() string {
	// First remove string after ":", which is the version
	name, _, _ := strings.CutLast(kit.AbsolutePath, ":")
	// Then split the last path component
	_, name = filepath.Split(name)
	return name
}

// STC = Standard Template Construct = AI model.
type STC struct {
	Model STCModel `yaml:"model"`
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
	setAbsoluteKitPaths(cfg.DefaultKits, homedir)

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

		setAbsoluteKitPaths(project.Kits, homedir)
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

	// Merge project-specific kits with default kits
	project.Kits = slices.Concat(cfg.DefaultKits, project.Kits)

	return project, nil
}

func setAbsoluteKitPaths(kits []Kit, homedir string) {
	for i := range kits {
		kits[i].AbsolutePath = ExpandHomedir(kits[i].Path, homedir)
	}
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
		DefaultKits: nil,
		Projects: []Project{{
			Path: filepath.Join("~", "example"),
			STC:  nil,
			Kits: nil,
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

type STCModel string

// Known values for [STC.Model]. This list is not exhaustive, so [STC.Model] may have values outside
// of this list.
//
// See "Available agents" at https://docs.docker.com/reference/cli/sbx/run/
const (
	STCClaude      STCModel = "claude"
	STCCodex       STCModel = "codex"
	STCCopilot     STCModel = "copilot"
	STCGemini      STCModel = "gemini"
	STCCursor      STCModel = "cursor"
	STCDevin       STCModel = "devin"
	STCDockerAgent STCModel = "docker-agent"
	STCDroid       STCModel = "droid"
	STCKiro        STCModel = "kiro"
)

// Provider returns the model provider for the STC, or ok=false if it's unknown.
//
// See "Available services" here: https://docs.docker.com/reference/cli/sbx/secret/set/
func (stc STCModel) Provider() (provider string, ok bool) {
	switch stc {
	case STCClaude:
		return "anthropic", true
	case STCCodex:
		return "openai", true
	case STCCopilot:
		return "github", true
	case STCGemini:
		return "google", true
	default:
		return "", false
	}
}

// V3 returns the kit-compatible V3 sbx workload for the built-in model:
// https://docs.docker.com/ai/sandboxes/customize/use-kits/#published-v3-workloads
//
// If we don't have an entry for the STC model, we assume it's already V3.
func (stc STCModel) V3() string {
	switch stc {
	case STCClaude:
		return "docker/sbx-kit-claude"
	case STCCodex:
		return "docker/sbx-kit-codex"
	case STCCopilot:
		return "docker/sbx-kit-copilot"
	case STCCursor:
		return "docker/sbx-kit-cursor"
	case STCDevin:
		return "docker/sbx-kit-devin"
	case STCDockerAgent:
		return "docker/sbx-kit-docker-agent"
	case STCDroid:
		return "docker/sbx-kit-droid"
	case STCKiro:
		return "docker/sbx-kit-kiro"
	default:
		return string(stc)
	}
}

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
