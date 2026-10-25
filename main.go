// Command impersonate picks a user in PropelAuth, by organization then by user, and
// impersonates them from the PropelAuth dashboard in a dedicated Chrome.
package main

import (
	"fmt"
	"os"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
)

// environment is a PropelAuth project, as declared by an [[envs]] table of config.toml.
type environment struct {
	Name         string `toml:"name"`
	AuthHostname string `toml:"auth_hostname"`
	// Secret is the AWS Secrets Manager secret holding the PropelAuth API key.
	Secret string `toml:"secret"`
	Color  string `toml:"color"`
}

// findEnv returns the index of the environment called name, -1 if there is none.
func findEnv(envs []environment, name string) int {
	return slices.IndexFunc(envs, func(e environment) bool { return e.Name == name })
}

func envNames(envs []environment) []string {
	names := make([]string, len(envs))
	for i, env := range envs {
		names[i] = env.Name
	}
	return names
}

func main() {
	path, err := settingsPath()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	// An invalid file only stops the commands that need it: config edit is how to fix it.
	defaults, settingsErr := loadSettings(path)
	if settingsErr == nil && len(defaults.Envs) == 0 {
		settingsErr = fmt.Errorf(`%s declares no environment: add them with impersonate config edit, for instance

[[envs]]
name = "staging"
auth_hostname = "auth.staging.example.com"
secret = "my-staging-secret"`, path)
	}

	root := rootCommand(defaults, settingsErr)
	root.AddCommand(configCommand(path))
	if root.Execute() != nil {
		os.Exit(1)
	}
}

func rootCommand(defaults settings, settingsErr error) *cobra.Command {
	var envName, awsProfile string
	var perPage int
	var showDisabled bool
	var chromeProfileName string
	cmd := &cobra.Command{
		Use:          "impersonate [organization search]",
		Short:        "Pick a PropelAuth user and impersonate them",
		Args:         cobra.ArbitraryArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			if settingsErr != nil {
				return settingsErr
			}
			o := options{envs: defaults.Envs, env: envName, awsProfile: awsProfile, query: strings.Join(args, " "), perPage: perPage, showDisabled: showDisabled}
			return run(o, chromeProfileName)
		},
	}
	cmd.Flags().StringVarP(&envName, "env", "e", defaults.Env, "environment of config.toml, picked in the TUI when empty")
	cmd.Flags().StringVarP(&awsProfile, "profile", "p", defaults.Profile, "AWS profile that reads the PropelAuth API key, the active one when empty")
	cmd.Flags().IntVar(&perPage, "per-page", defaults.perPage(), "items per page, 0 to fill the terminal")
	cmd.Flags().BoolVar(&showDisabled, "show-disabled", defaults.ShowDisabled, "list the disabled and locked users, which d toggles in the TUI")
	cmd.Flags().StringVar(&chromeProfileName, "chrome-profile", defaults.ChromeProfile, "Chrome profile to open the impersonation in, without the automatic click; the dedicated one when empty")
	_ = cmd.RegisterFlagCompletionFunc("env", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return envNames(defaults.Envs), cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func run(o options, chromeProfileName string) error {
	if o.env != "" && findEnv(o.envs, o.env) < 0 {
		return fmt.Errorf("unknown environment %q", o.env)
	}
	chrome, err := lookupChromeProfile(chromeProfileName)
	if err != nil {
		return err
	}
	o.chrome = chrome
	m := newModel(o)
	final, err := tea.NewProgram(m).Run()
	if err != nil {
		return err
	}
	m = final.(model)
	if m.err != nil {
		return m.err
	}
	if m.done != "" {
		fmt.Println(m.done)
	}
	return nil
}
