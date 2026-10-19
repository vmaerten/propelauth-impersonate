package main

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	defaultPerPage = 10
	// defaultEnvColor is the list title's own background.
	defaultEnvColor = "62"
)

// settings mirrors config.toml: what it leaves out falls back to the built-in defaults.
type settings struct {
	Env          string `toml:"env"`
	Profile      string `toml:"profile"`
	PerPage      *int   `toml:"per_page"`
	ShowDisabled bool   `toml:"show_disabled"`
	// ChromeProfile names one of the user's Chrome profiles, the dedicated one being the default.
	ChromeProfile string        `toml:"chrome_profile"`
	Envs          []environment `toml:"envs"`
}

func (s settings) perPage() int {
	if s.PerPage == nil {
		return defaultPerPage
	}
	return *s.PerPage
}

// settingsPath is $XDG_CONFIG_HOME/impersonate/config.toml, ~/.config by default.
func settingsPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "impersonate", "config.toml"), nil
}

// readSettingsFile returns the content of path, empty when it does not exist.
func readSettingsFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

func loadSettings(path string) (settings, error) {
	data, err := readSettingsFile(path)
	if err != nil {
		return settings{}, err
	}
	s, err := parseSettings(data)
	if err != nil {
		return s, fmt.Errorf("%s: %w", path, err)
	}
	return s, nil
}

func parseSettings(data []byte) (settings, error) {
	var s settings
	if err := toml.NewDecoder(bytes.NewReader(data)).DisallowUnknownFields().Decode(&s); err != nil {
		if strict, ok := errors.AsType[*toml.StrictMissingError](err); ok {
			return s, fmt.Errorf("unknown setting %s", strings.Join(strict.Errors[0].Key(), "."))
		}
		return s, err
	}
	for i, env := range s.Envs {
		switch {
		case env.Name == "":
			return s, fmt.Errorf("environment %d has no name", i+1)
		case findEnv(s.Envs[:i], env.Name) >= 0:
			return s, fmt.Errorf("environment %q is declared twice", env.Name)
		case env.AuthHostname == "" || env.Secret == "":
			return s, fmt.Errorf("environment %q needs an auth_hostname and a secret", env.Name)
		}
		s.Envs[i].Color = cmp.Or(env.Color, defaultEnvColor)
	}
	if s.Env != "" && findEnv(s.Envs, s.Env) < 0 {
		return s, fmt.Errorf("env: unknown environment %q", s.Env)
	}
	return s, nil
}

// writeSetting sets key to the TOML value literal in the file at path, or removes it when
// literal is nil, leaving the rest of the file as it is.
func writeSetting(path, key string, literal *string) error {
	data, err := readSettingsFile(path)
	if err != nil {
		return err
	}
	if _, err := parseSettings(data); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	edited, err := editTOML(data, key, literal)
	if err != nil {
		return err
	}
	if _, err := parseSettings(edited); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, edited, 0o644)
}

// scalarKeys are the settings config set and unset edit in place.
var scalarKeys = []string{"env", "profile", "per_page", "show_disabled", "chrome_profile"}

// keys lists the settings of s in config.toml order, its environments included.
func (s settings) keys() []string {
	keys := slices.Clone(scalarKeys)
	for _, env := range s.Envs {
		for _, field := range []string{"auth_hostname", "secret", "color"} {
			keys = append(keys, "envs."+env.Name+"."+field)
		}
	}
	return keys
}

// writableKey checks that config set and unset can edit key in place.
func writableKey(key string) error {
	if strings.HasPrefix(key, "envs.") {
		return errors.New("the environments change through impersonate config edit")
	}
	if !slices.Contains(scalarKeys, key) {
		return fmt.Errorf("unknown setting %q", key)
	}
	return nil
}

// settingLiteral checks value for key and returns it as a TOML value.
func settingLiteral(key, value string) (string, error) {
	if err := writableKey(key); err != nil {
		return "", err
	}
	switch key {
	case "per_page":
		n, err := strconv.Atoi(value)
		if err != nil {
			return "", fmt.Errorf("per_page: %q is not a number", value)
		}
		return strconv.Itoa(n), nil
	case "show_disabled":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return "", fmt.Errorf("show_disabled: %q is not true or false", value)
		}
		return strconv.FormatBool(b), nil
	}
	// Go's quoting matches TOML basic strings for the printable values settings hold.
	return strconv.Quote(value), nil
}

// value returns the effective value of key, false when it has none.
func (s settings) value(key string) (string, bool) {
	if rest, ok := strings.CutPrefix(key, "envs."); ok {
		dot := strings.LastIndex(rest, ".")
		if dot < 0 {
			return "", false
		}
		i := findEnv(s.Envs, rest[:dot])
		if i < 0 {
			return "", false
		}
		switch rest[dot+1:] {
		case "auth_hostname":
			return s.Envs[i].AuthHostname, true
		case "secret":
			return s.Envs[i].Secret, true
		case "color":
			return s.Envs[i].Color, true
		}
		return "", false
	}
	switch key {
	case "env":
		return s.Env, s.Env != ""
	case "profile":
		return s.Profile, s.Profile != ""
	case "per_page":
		return strconv.Itoa(s.perPage()), true
	case "show_disabled":
		return strconv.FormatBool(s.ShowDisabled), true
	case "chrome_profile":
		return s.ChromeProfile, s.ChromeProfile != ""
	}
	return "", false
}
