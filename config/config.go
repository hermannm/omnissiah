package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"strings"

	"github.com/goccy/go-yaml"
	"hermannm.dev/errclose"
	"hermannm.dev/wrap"
)

type Config struct {
	DefaultModel string    `yaml:"defaultModel,omitempty"`
	Projects     []Project `yaml:"projects"`
}

type Project struct {
	Path  string `yaml:"path"`
	Model string `yaml:"model,omitempty"`
}

const configPath = "~/omnissiah.yml"

func Load() (cfg Config, returnedErr error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return Config{}, wrap.Error(err, "could not get user home dir")
	}
	absoluteConfigPath := expandHomedir(configPath, homedir)

	configFile, err := os.Open(absoluteConfigPath)
	if err != nil {
		return Config{}, wrap.ErrorWithAttrs(
			err,
			"failed to open config file",
			"path", configPath,
		)
	}
	defer errclose.Close(configFile, &returnedErr, "config file")

	if err := yaml.NewDecoder(configFile).Decode(&cfg); err != nil {
		return Config{}, wrap.ErrorWithAttrs(
			err,
			"failed to parse config file as YAML",
			"path", configPath,
		)
	}

	// Expand uses of ~ in project paths
	for i, project := range cfg.Projects {
		cfg.Projects[i].Path = expandHomedir(project.Path, homedir)

		if project.Model == "" && cfg.DefaultModel == "" {
			return Config{}, wrap.ErrorWithAttrs(
				err,
				"either 'defaultModel' or 'project.model' must be set in config",
				"path", configPath,
			)
		}
	}

	return cfg, nil
}

func (cfg Config) GetCurrentProject() (Project, error) {
	path, err := os.Getwd()
	if err != nil {
		return Project{}, wrap.Error(err, "failed to get current directory")
	}

	i := slices.IndexFunc(
		cfg.Projects,
		func(p Project) bool {
			return strings.HasPrefix(path, p.Path)
		},
	)
	if i < 0 {
		return Project{}, fmt.Errorf(
			"'%s' is not a configured project path in %s",
			path, configPath,
		)
	}
	project := cfg.Projects[i]

	if project.Model == "" {
		project.Model = cfg.DefaultModel
	}

	return project, nil
}

func Initialize() (returnErr error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return wrap.Error(err, "could not get user home dir")
	}
	absoluteConfigPath := expandHomedir(configPath, homedir)

	if fileExists(absoluteConfigPath) {
		slog.Info(fmt.Sprintf("%s already exists, skipping initialization", configPath))
		return nil
	}

	file, err := os.Create(absoluteConfigPath)
	if err != nil {
		return wrap.ErrorWithAttrs(
			err,
			"failed to create Omnissiah config file",
			"path", configPath,
		)
	}
	defer errclose.Closef(file, &returnErr, "config file %s", configPath)

	encoder := yaml.NewEncoder(file)
	defer errclose.Close(encoder, &returnErr, "YAML encoder")

	startingConfig := Config{
		DefaultModel: "claude",
		Projects:     []Project{{Path: "~/example", Model: ""}},
	}

	if err := encoder.Encode(startingConfig); err != nil {
		return wrap.Error(err, "failed to encode config YAML")
	}

	slog.Info("Initialized Omnissiah config", "path", configPath, "config", startingConfig)
	return nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return !errors.Is(err, fs.ErrNotExist)
}

func expandHomedir(path string, homedir string) (expandedPath string) {
	return strings.Replace(path, "~", homedir, 1)
}
