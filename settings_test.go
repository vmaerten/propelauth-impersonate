package main

import (
	"os"
	"path/filepath"
	"testing"
)

const twoEnvs = `
[[envs]]
name = "staging"
auth_hostname = "auth.staging.example.com"
secret = "staging-secret"
color = "#B9770E"

[[envs]]
name = "prod"
auth_hostname = "auth.example.com"
secret = "prod-secret"
`

func writeSettings(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadSettings(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		s, err := loadSettings(filepath.Join(t.TempDir(), "config.toml"))
		if err != nil || s.Env != "" || s.Profile != "" || s.perPage() != 10 || len(s.Envs) != 0 {
			t.Errorf("got %+v, %v; want the built-in defaults", s, err)
		}
	})

	t.Run("file over the defaults", func(t *testing.T) {
		s, err := loadSettings(writeSettings(t, "env = \"prod\"\nper_page = 0\n"+twoEnvs))
		if err != nil {
			t.Fatal(err)
		}
		if s.Env != "prod" || s.Profile != "" || s.perPage() != 0 {
			t.Errorf("got %+v", s)
		}
		if names := envNames(s.Envs); len(names) != 2 || names[0] != "staging" || names[1] != "prod" {
			t.Errorf("environments %v, want staging then prod as in the file", names)
		}
		if color, _ := s.value("envs.prod.color"); color != defaultEnvColor {
			t.Errorf("prod without a color got %q, want the default %q", color, defaultEnvColor)
		}
	})

	for name, content := range map[string]string{
		"unknown setting":            "per-page = 5\n",
		"former colors table":        "[colors]\nprod = \"#FF0000\"\n",
		"undeclared environment":     "env = \"prod\"\n",
		"environment field":          "[[envs]]\nname = \"prod\"\nauth_hostname = \"a\"\nsecret = \"s\"\ncolour = \"red\"\n",
		"environment without name":   "[[envs]]\nauth_hostname = \"a\"\nsecret = \"s\"\n",
		"environment without secret": "[[envs]]\nname = \"prod\"\nauth_hostname = \"a\"\n",
		"environment declared twice": twoEnvs + "[[envs]]\nname = \"prod\"\nauth_hostname = \"a\"\nsecret = \"s\"\n",
		"unquoted color":             "[[envs]]\nname = \"prod\"\nauth_hostname = \"a\"\nsecret = \"s\"\ncolor = #FF0000\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := loadSettings(writeSettings(t, content)); err == nil {
				t.Error("loaded without an error")
			}
		})
	}
}

func TestEditTOML(t *testing.T) {
	const envs = "\n# Environments\n[[envs]]\nname = \"staging\"\nauth_hostname = \"a\"\nsecret = \"s\" # Secrets Manager\n"
	const commented = "# My defaults\nenv = \"staging\"   # safer\nper_page   =  10\n" + envs
	value := func(s string) *string { return &s }
	for _, tc := range []struct {
		name, data, key string
		literal         *string
		want            string
	}{
		{"replace a value", commented, "per_page", value("0"),
			"# My defaults\nenv = \"staging\"   # safer\nper_page   =  0\n" + envs},
		{"remove a line", commented, "env", nil, "# My defaults\nper_page   =  10\n" + envs},
		{"add after the last top-level key", commented, "profile", value(`"admin"`),
			"# My defaults\nenv = \"staging\"   # safer\nper_page   =  10\nprofile = \"admin\"\n" + envs},
		{"add before the first table", envs, "per_page", value("0"), "per_page = 0\n" + envs},
		{"add to an empty file", "", "env", value(`"staging"`), "env = \"staging\"\n"},
		{"add after a last line without newline", `per_page = 0`, "profile", value(`"admin"`), "per_page = 0\nprofile = \"admin\"\n"},
		{"remove a missing key", commented, "profile", nil, commented},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := editTOML([]byte(tc.data), tc.key, tc.literal)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.want {
				t.Errorf("got\n%s\nwant\n%s", got, tc.want)
			}
		})
	}
}

func TestWriteSetting(t *testing.T) {
	path := writeSettings(t, twoEnvs)
	zero := "0"
	if err := writeSetting(path, "per_page", &zero); err != nil {
		t.Fatal(err)
	}
	if s, err := loadSettings(path); err != nil || s.perPage() != 0 {
		t.Errorf("per_page %d, %v once set to 0", s.perPage(), err)
	}
	if err := writeSetting(path, "per_page", nil); err != nil {
		t.Fatal(err)
	}
	if s, err := loadSettings(path); err != nil || s.perPage() != 10 {
		t.Errorf("per_page %d, %v once unset, want the default 10", s.perPage(), err)
	}

	t.Run("refuses an undeclared environment", func(t *testing.T) {
		path := writeSettings(t, twoEnvs)
		dev := `"dev"`
		if err := writeSetting(path, "env", &dev); err == nil {
			t.Error("set env to an undeclared environment")
		}
		if content, _ := os.ReadFile(path); string(content) != twoEnvs {
			t.Errorf("the file changed:\n%s", content)
		}
	})
}

func TestSettingLiteral(t *testing.T) {
	if got, err := settingLiteral("profile", `my "admin"`); err != nil || got != `"my \"admin\""` {
		t.Errorf("profile literal %s, %v", got, err)
	}
	for key, value := range map[string]string{"per_page": "ten", "show_disabled": "maybe", "envs.prod.color": "#FF0000", "colour": "#FF0000"} {
		if _, err := settingLiteral(key, value); err == nil {
			t.Errorf("accepted %s = %s", key, value)
		}
	}
}
