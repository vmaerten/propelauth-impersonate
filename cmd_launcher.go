package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

// orgUser is a user of the users command, with their role in its organization.
type orgUser struct {
	ID        string `json:"user_id"`
	Email     string `json:"email"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Role      string `json:"role"`
	Enabled   bool   `json:"enabled"`
	Locked    bool   `json:"locked"`
}

// launcherCommands are what a launcher such as Raycast runs in place of the TUI, each step of
// it on its own.
func launcherCommands(defaults settings, settingsErr error) []*cobra.Command {
	envs := &cobra.Command{
		Use:   "envs",
		Short: "Print the environments as JSON",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if settingsErr != nil {
				return settingsErr
			}
			return printJSON(cmd, defaults.Envs)
		},
	}

	orgs := &cobra.Command{
		Use:   "orgs",
		Short: "Print the organizations of an environment as JSON",
		Args:  cobra.NoArgs,
	}
	orgsClient := clientFlags(orgs, defaults, settingsErr)
	orgs.RunE = func(cmd *cobra.Command, _ []string) error {
		client, err := orgsClient(cmd.Context())
		if err != nil {
			return err
		}
		orgs, err := client.orgs(cmd.Context())
		if err != nil {
			return err
		}
		return printJSON(cmd, orgs)
	}

	users := &cobra.Command{
		Use:   "users <org-id>",
		Short: "Print the users of an organization as JSON, the disabled and locked ones included",
		Args:  cobra.ExactArgs(1),
	}
	usersClient := clientFlags(users, defaults, settingsErr)
	users.RunE = func(cmd *cobra.Command, args []string) error {
		client, err := usersClient(cmd.Context())
		if err != nil {
			return err
		}
		orgID := args[0]
		found, err := client.usersInOrg(cmd.Context(), orgID)
		if err != nil {
			return err
		}
		out := make([]orgUser, len(found))
		for i, u := range found {
			out[i] = orgUser{
				ID:        u.ID,
				Email:     u.Email,
				FirstName: u.FirstName,
				LastName:  u.LastName,
				Role:      u.Orgs[orgID].Role,
				Enabled:   u.Enabled,
				Locked:    u.Locked,
			}
		}
		return printJSON(cmd, out)
	}

	open := &cobra.Command{
		Use:   "open <user-id>",
		Short: "Impersonate a user, as picking them in the TUI does",
		Args:  cobra.ExactArgs(1),
	}
	openEnv := envFlag(open, defaults, settingsErr)
	chromeProfileName := open.Flags().String("chrome-profile", defaults.ChromeProfile, "Chrome profile to open the impersonation in, without the automatic click; the dedicated one when empty")
	notify := open.Flags().Bool("notify", false, "report a failure in a macOS notification, for a launcher that does not wait for the command")
	open.RunE = func(cmd *cobra.Command, args []string) error {
		err := openUser(cmd.Context(), openEnv, *chromeProfileName, args[0])
		if err != nil && *notify {
			err = errors.Join(err, notifyFailure(err))
		}
		return err
	}

	return []*cobra.Command{envs, orgs, users, open}
}

// envFlag adds --env to cmd, and returns how to look up the environment it names.
func envFlag(cmd *cobra.Command, defaults settings, settingsErr error) func() (environment, error) {
	name := cmd.Flags().StringP("env", "e", defaults.Env, "environment of config.toml")
	_ = cmd.RegisterFlagCompletionFunc("env", completeEnvs(defaults.Envs))
	return func() (environment, error) {
		if settingsErr != nil {
			return environment{}, settingsErr
		}
		if *name == "" {
			return environment{}, errors.New("no environment: pass --env, or set env in the config")
		}
		i := findEnv(defaults.Envs, *name)
		if i < 0 {
			return environment{}, fmt.Errorf("unknown environment %q", *name)
		}
		return defaults.Envs[i], nil
	}
}

// clientFlags adds --env and --profile to cmd, and returns how to get a client of that
// environment.
func clientFlags(cmd *cobra.Command, defaults settings, settingsErr error) func(context.Context) (*propelAuth, error) {
	env := envFlag(cmd, defaults, settingsErr)
	awsProfile := cmd.Flags().StringP("profile", "p", defaults.Profile, "AWS profile that reads the PropelAuth API key, the active one when empty")
	return func(ctx context.Context) (*propelAuth, error) {
		e, err := env()
		if err != nil {
			return nil, err
		}
		return newPropelAuth(ctx, e, *awsProfile)
	}
}

// openUser impersonates userID in the environment that env looks up.
func openUser(ctx context.Context, env func() (environment, error), chromeProfileName, userID string) error {
	e, err := env()
	if err != nil {
		return err
	}
	chrome, err := lookupChromeProfile(chromeProfileName)
	if err != nil {
		return err
	}
	return openImpersonation(ctx, e, userID, chrome)
}

// notifyFailure shows failure in a macOS notification.
func notifyFailure(failure error) error {
	// The message goes as an argument, out of reach of the script's quoting.
	script := []string{"-e", "on run argv", "-e", `display notification (item 1 of argv) with title "Impersonation failed"`, "-e", "end run"}
	if out, err := exec.Command("osascript", append(script, failure.Error())...).CombinedOutput(); err != nil {
		return fmt.Errorf("notifying the failure: %w: %s", err, out)
	}
	return nil
}

func printJSON(cmd *cobra.Command, v any) error {
	return json.NewEncoder(cmd.OutOrStdout()).Encode(v)
}
