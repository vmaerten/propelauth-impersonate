package main

import (
	"cmp"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func configCommand(path string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Read and write the settings in " + path,
	}
	loaded := func() settings {
		s, _ := loadSettings(path)
		return s
	}
	completeKey := func(keys func() []string) cobra.CompletionFunc {
		return func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
			switch {
			case len(args) == 0:
				return keys(), cobra.ShellCompDirectiveNoFileComp
			case len(args) == 1 && args[0] == "env":
				return envNames(loaded().Envs), cobra.ShellCompDirectiveNoFileComp
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
	}
	allKeys := func() []string { return loaded().keys() }
	writableKeys := func() []string { return scalarKeys }
	cmd.AddCommand(
		&cobra.Command{
			Use:   "list",
			Short: "Print the effective settings, file and built-in defaults",
			Args:  cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				s, err := loadSettings(path)
				if err != nil {
					return err
				}
				for _, key := range s.keys() {
					if value, ok := s.value(key); ok {
						fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", key, value)
					}
				}
				return nil
			},
		},
		&cobra.Command{
			Use:               "get <key>",
			Short:             "Print the effective value of a setting",
			Args:              cobra.ExactArgs(1),
			ValidArgsFunction: completeKey(allKeys),
			RunE: func(cmd *cobra.Command, args []string) error {
				s, err := loadSettings(path)
				if err != nil {
					return err
				}
				value, ok := s.value(args[0])
				if !ok {
					return fmt.Errorf("%s is not set", args[0])
				}
				fmt.Fprintln(cmd.OutOrStdout(), value)
				return nil
			},
		},
		&cobra.Command{
			Use:               "set <key> <value>",
			Short:             "Write a setting, leaving the rest of the file as it is",
			Args:              cobra.ExactArgs(2),
			ValidArgsFunction: completeKey(writableKeys),
			RunE: func(_ *cobra.Command, args []string) error {
				literal, err := settingLiteral(args[0], args[1])
				if err != nil {
					return err
				}
				return writeSetting(path, args[0], &literal)
			},
		},
		&cobra.Command{
			Use:               "unset <key>",
			Short:             "Remove a setting, leaving the rest of the file as it is",
			Args:              cobra.ExactArgs(1),
			ValidArgsFunction: completeKey(writableKeys),
			RunE: func(_ *cobra.Command, args []string) error {
				if err := writableKey(args[0]); err != nil {
					return err
				}
				return writeSetting(path, args[0], nil)
			},
		},
		&cobra.Command{
			Use:   "edit",
			Short: "Open the file in $VISUAL or $EDITOR, then check it",
			Args:  cobra.NoArgs,
			RunE: func(*cobra.Command, []string) error {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					return err
				}
				// Through sh, like git, for editors set with arguments such as "code --wait".
				editor := cmp.Or(os.Getenv("VISUAL"), os.Getenv("EDITOR"), "vi")
				edit := exec.Command("sh", "-c", editor+` "$1"`, "sh", path)
				edit.Stdin, edit.Stdout, edit.Stderr = os.Stdin, os.Stdout, os.Stderr
				if err := edit.Run(); err != nil {
					return err
				}
				_, err := loadSettings(path)
				return err
			},
		},
		&cobra.Command{
			Use:   "path",
			Short: "Print the path of the file",
			Args:  cobra.NoArgs,
			Run: func(cmd *cobra.Command, _ []string) {
				fmt.Fprintln(cmd.OutOrStdout(), path)
			},
		},
	)
	return cmd
}
