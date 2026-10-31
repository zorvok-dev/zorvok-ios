// Package config manages Zorvok's global (~/.config/zorvok) and
// per-project (.zorvok.yml) configuration files.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Config is a flat key/value settings store, serialized as YAML.
// Milestone 1 scope is deliberately generic (arbitrary string settings,
// e.g. "email", "default_registry") rather than a fully-typed schema:
// structured registry/auth config lands with Milestones 3-5 once there
// is a real registry API to shape it around.
type Config struct {
	Values map[string]string `yaml:",inline"`
	path   string            `yaml:"-"`
}

// GlobalDir returns the directory holding Zorvok's global config and
// credentials, creating it if necessary.
func GlobalDir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("determine user config dir: %w", err)
	}
	dir := filepath.Join(base, "zorvok")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create config dir %s: %w", dir, err)
	}
	return dir, nil
}

// GlobalPath returns the path to the global config file.
func GlobalPath() (string, error) {
	dir, err := GlobalDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.yml"), nil
}

// ProjectPath returns the path to the project-local config file for dir.
func ProjectPath(dir string) string {
	return filepath.Join(dir, ".zorvok.yml")
}

// Load reads a config file at path. A missing file is not an error; it
// yields an empty Config, matching the YAML files' optional nature.
func Load(path string) (*Config, error) {
	c := &Config{Values: map[string]string{}, path: path}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return c, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if len(data) > 0 {
		if err := yaml.Unmarshal(data, &c.Values); err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
	}
	if c.Values == nil {
		c.Values = map[string]string{}
	}
	return c, nil
}

// Save writes the config back to its path.
func (c *Config) Save() error {
	if c.path == "" {
		return fmt.Errorf("config has no associated path")
	}
	data, err := yaml.Marshal(c.Values)
	if err != nil {
		return fmt.Errorf("encode config: %w", err)
	}
	if err := os.WriteFile(c.path, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", c.path, err)
	}
	return nil
}

// Get returns a key's value and whether it was set.
func (c *Config) Get(key string) (string, bool) {
	v, ok := c.Values[key]
	return v, ok
}

// Set assigns a key's value.
func (c *Config) Set(key, value string) {
	if c.Values == nil {
		c.Values = map[string]string{}
	}
	c.Values[key] = value
}

// Unset removes a key.
func (c *Config) Unset(key string) {
	delete(c.Values, key)
}

// Keys returns all set keys, sorted.
func (c *Config) Keys() []string {
	keys := make([]string, 0, len(c.Values))
	for k := range c.Values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Merged layers project config over global config: project-local
// settings win, matching the usual "closest scope wins" convention.
func Merged(global, project *Config) *Config {
	m := &Config{Values: map[string]string{}}
	for k, v := range global.Values {
		m.Values[k] = v
	}
	for k, v := range project.Values {
		m.Values[k] = v
	}
	return m
}
