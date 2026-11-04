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
	DefaultModel *Model `yaml:"defaultModel,omitempty"`
	// sbx kits to apply to all Omnissiah prayers:
	// https://docs.docker.com/ai/sandboxes/customize/use-kits/
	DefaultKits     []Kit     `yaml:"defaultKits,omitempty"`
	AllowedNetworks []Network `yaml:"allowedNetworks,omitempty"`
	Projects        []Project `yaml:"projects"`
	// Omitted from YAML, since we set this ourselves.
	//exhaustruct:optional
	Path Path `yaml:"-"`
	// We include the user's home directory in the config struct after parsing, so that we don't
	// have to call [os.UserHomeDir] again later.
	//exhaustruct:optional
	Homedir string `yaml:"-"`
}

type Project struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
	// Populated in [Load].
	//exhaustruct:optional
	AbsolutePath string `yaml:"-"`
	Model        *Model `yaml:"model,omitempty"`
	// sbx kits to apply to Omnissiah prayers in this project
	Kits      []Kit              `yaml:"kits,omitempty"`
	Bootstrap []BootstrapCommand `yaml:"bootstrap,omitempty"`
}

func (project Project) TemplateName() string {
	// Use omnissiah prefix to avoid conflicts
	return "omnissiah-" + project.Name
}

// sbx saves local templates with a "docker.io/library/" prefix, and a "latest" tag by default.
func (project Project) FullTemplateTag() string {
	return "docker.io/library/" + project.TemplateName() + ":latest"
}

type Kit struct {
	// From sbx docs: "directory, ZIP, git, or OCI"
	Path string `yaml:"path"`
	// Set with [setAbsoluteKitPaths].
	//exhaustruct:optional
	AbsolutePath string `yaml:"-"`
	// Arguments to pass to the kit (Docker build args).
	Args []string `yaml:"args,omitempty"`
	// Host directories to mount into the sandbox, if the kit declares a host-mount capability.
	// Format: HOST:SANDBOX_PATH
	// https://docs.docker.com/ai/sandboxes/customize/use-kits/#choose-directories-for-kit-host-mounts
	Mounts []string `yaml:"mounts,omitempty"`
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

// BootstrapCommand must contain either [BootstrapCommand.Cmd] OR [BootstrapCommand.Cp].
type BootstrapCommand struct {
	//exhaustruct:optional
	Cmd string `yaml:"cmd,omitempty"`
	//exhaustruct:optional
	Cp *BootstrapCopy `yaml:"cp,omitempty"`
}

func (cmd BootstrapCommand) validate() error {
	if cmd.Cmd == "" && cmd.Cp == nil {
		return errors.New("bootstrap command must have either 'cmd' or 'cp' field")
	}
	if cmd.Cmd != "" && cmd.Cp != nil {
		return errors.New("bootstrap command cannot have both 'cmd' and 'cp' fields")
	}
	return nil
}

type BootstrapCopy struct {
	// Host path.
	From string `yaml:"from"`
	// Same as [BootstrapCopy.From], but with `~` expanded to the user's home dir.
	FromAbsolute string `yaml:"-"`
	// Sandbox path.
	To string `yaml:"to"`
	// Same as [BootstrapCopy.To], but with `~` expanded to `/home/agent`.
	ToAbsolute string `yaml:"-"`
}

// The home directory for agents in Docker sandboxes:
// https://docs.docker.com/ai/sandboxes/customize/author/build-an-agent/#create-the-agent-account
const agentHomedir = "/home/agent"

func (cp *BootstrapCopy) setAbsolutePaths(userHomedir string) {
	cp.FromAbsolute = ExpandHomedir(cp.From, userHomedir)
	cp.ToAbsolute = ExpandHomedir(cp.To, agentHomedir)
}

type Model struct {
	Name ModelName `yaml:"name"`
	Args []string  `yaml:"args,omitempty"`
}

type Network struct {
	Host    string   `yaml:"host"`
	Methods []string `yaml:"methods,omitempty"`
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

	for i, project := range cfg.Projects {
		cfg.Projects[i].AbsolutePath = ExpandHomedir(project.Path, homedir)
		setAbsoluteKitPaths(project.Kits, homedir)

		if project.Model == nil && cfg.DefaultModel == nil {
			return Config{}, wrap.ErrorWithAttrs(
				err,
				"either 'defaultModel' or 'project.model' must be set in config",
				"path", configPath.Short,
			)
		}

		for j, cmd := range project.Bootstrap {
			if err := cmd.validate(); err != nil {
				return Config{}, wrap.ErrorWithAttrs(
					err,
					"invalid bootstrap command",
					"project", project.Name,
					"index", j,
				)
			}

			if cmd.Cp != nil {
				cfg.Projects[i].Bootstrap[j].Cp.setAbsolutePaths(homedir)
			}
		}
	}

	return cfg, nil
}

func (cfg Config) GetProject(projectName string) (Project, error) {
	project, ok := sliceutil.Find(
		cfg.Projects,
		func(project Project) bool {
			return project.Name == projectName
		},
	)
	if !ok {
		return Project{}, fmt.Errorf("no project configured with name '%s'", projectName)
	}

	return project.withDefaultConfig(cfg), nil
}

func (cfg Config) GetCurrentProject() (Project, error) {
	path, err := os.Getwd()
	if err != nil {
		return Project{}, wrap.Error(err, "failed to get current directory to determine project")
	}

	project, ok := sliceutil.Find(
		cfg.Projects,
		func(p Project) bool {
			return strings.HasPrefix(path, p.AbsolutePath)
		},
	)
	if !ok {
		return Project{}, fmt.Errorf(
			"'%s' is not a configured project path in %s",
			CollapseHomedir(path, cfg.Homedir), cfg.Path.Short,
		)
	}

	return project.withDefaultConfig(cfg), nil
}

func (project Project) withDefaultConfig(cfg Config) Project {
	if project.Model == nil {
		project.Model = cfg.DefaultModel
	}

	// Merge project-specific kits with default kits
	project.Kits = slices.Concat(cfg.DefaultKits, project.Kits)

	return project
}

// Populates [Kit.AbsolutePath], and also expands homedir (~) in mount paths.
func setAbsoluteKitPaths(kits []Kit, homedir string) {
	for i, kit := range kits {
		kits[i].AbsolutePath = ExpandHomedir(kit.Path, homedir)

		for j, mount := range kit.Mounts {
			kits[i].Mounts[j] = ExpandHomedir(mount, homedir)
		}
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

	encoder := yaml.NewEncoder(configFile, yaml.IndentSequence(true))
	defer errclose.Close(encoder, &returnErr, "YAML encoder")

	startingConfig := Config{
		DefaultModel: new(
			Model{
				Name: "claude",
				Args: []string{"--model=opus"},
			},
		),
		DefaultKits: nil,
		AllowedNetworks: []Network{
			{Host: "registry.npmjs.org", Methods: []string{"GET"}},
		},
		Projects: []Project{{
			Name:      "example",
			Path:      filepath.Join("~", "example"),
			Model:     nil,
			Kits:      nil,
			Bootstrap: nil,
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

type ModelName string

// Known values for [Model.Name]. This list is not exhaustive, so [Model.Name] may have values
// outside
// of this list.
//
// See "Available agents" at https://docs.docker.com/reference/cli/sbx/run/
const (
	ModelClaude      ModelName = "claude"
	ModelCodex       ModelName = "codex"
	ModelCopilot     ModelName = "copilot"
	ModelGemini      ModelName = "gemini"
	ModelCursor      ModelName = "cursor"
	ModelDevin       ModelName = "devin"
	ModelDockerAgent ModelName = "docker-agent"
	ModelDroid       ModelName = "droid"
	ModelKiro        ModelName = "kiro"
)

// Provider returns the model provider for the Model, or ok=false if it's unknown.
//
// See "Available services" here: https://docs.docker.com/reference/cli/sbx/secret/set/
//
//goland:noinspection GoUnusedConst
func (model ModelName) Provider() (provider string, ok bool) {
	switch model {
	case ModelClaude:
		return "anthropic", true
	case ModelCodex:
		return "openai", true
	case ModelCopilot:
		return "github", true
	case ModelGemini:
		return "google", true
	default:
		return "", false
	}
}

// V3 returns the kit-compatible v3 sbx workload for the given built-in model:
// https://docs.docker.com/ai/sandboxes/customize/use-kits/#published-v3-workloads
//
// If we don't have an entry for the Model model, we assume it's already V3.
func (model ModelName) V3() string {
	switch model {
	case ModelClaude:
		return "docker/sbx-kit-claude"
	case ModelCodex:
		return "docker/sbx-kit-codex"
	case ModelCopilot:
		return "docker/sbx-kit-copilot"
	case ModelCursor:
		return "docker/sbx-kit-cursor"
	case ModelDevin:
		return "docker/sbx-kit-devin"
	case ModelDockerAgent:
		return "docker/sbx-kit-docker-agent"
	case ModelDroid:
		return "docker/sbx-kit-droid"
	case ModelKiro:
		return "docker/sbx-kit-kiro"
	default:
		return string(model)
	}
}

// Matches checks if the model matches the given known model, by checking if the model name contains
// the known model name (so e.g. "docker/sbx-kit-claude" matches "claude").
func (model ModelName) Matches(knownModel ModelName) bool {
	return strings.Contains(string(model), string(knownModel))
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
