package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

type Local struct {
	Name string `toml:"name"`
	Path string `toml:"path"`
}

type Config struct {
	Server     string   `toml:"server"`
	Port       string   `toml:"port"`
	RemotePath string   `toml:"remote_path"`
	Excludes   []string `toml:"excludes"`
	Locals     []Local  `toml:"locals"`
}

func (c *Config) RemoteForLocal(l *Local) string {
	return fmt.Sprintf("%s:%s/%s", c.Server, c.RemotePath, l.Name)
}

func configDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "gs")
}

func configPath() string {
	return filepath.Join(configDir(), "gs.toml")
}

func loadConfig() (*Config, error) {
	path := configPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	for i := range cfg.Locals {
		cfg.Locals[i].Path = expandPath(cfg.Locals[i].Path)
	}

	return &cfg, nil
}

func (c *Config) FindLocalForPath(path string) *Local {
	path = expandPath(path)
	for i := range c.Locals {
		if path == c.Locals[i].Path || strings.HasPrefix(path, c.Locals[i].Path+string(filepath.Separator)) {
			return &c.Locals[i]
		}
	}
	return nil
}

func (c *Config) FindLocalByName(name string) *Local {
	for i := range c.Locals {
		if c.Locals[i].Name == name {
			return &c.Locals[i]
		}
	}
	return nil
}

func (c *Config) AddLocal(name, path string) error {
    // we shouldn't allow locals with same name due to potential conflicts at remote
    // e.g. '~/documents/pdfs' and '~/notes/pdfs' would both be stored as '<remote>/pdfs'
	if existing := c.FindLocalByName(name); existing != nil {
		return fmt.Errorf("local '%s' already exists (names must be unique across all tracked directories)", name)
	}
	if existing := c.FindLocalForPath(path); existing != nil {
		return fmt.Errorf("path '%s' already configured as local '%s'", path, existing.Name)
	}
	c.Locals = append(c.Locals, Local{Name: name, Path: path})
	return nil
}

func (c *Config) RemoveLocal(name string) {
	for i := range c.Locals {
		if c.Locals[i].Name == name {
			c.Locals = append(c.Locals[:i], c.Locals[i+1:]...)
			return
		}
	}
}

func saveConfig(cfg *Config) error {
	dir := configDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	f, err := os.Create(configPath())
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

func parseRemote(remote string) (host, port, path string, err error) {
	parts := strings.SplitN(remote, ":", 3)
	if len(parts) != 3 {
		return "", "", "", fmt.Errorf("invalid remote format, expected 'user@host:port:/path'")
	}
	return parts[0], parts[1], strings.TrimSuffix(parts[2], "/"), nil
}

