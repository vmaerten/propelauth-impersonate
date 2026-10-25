package main

import (
	"io"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// An environment the config lacks would reach AWS, or open the dashboard on no auth domain.
func TestLauncherCommandsNeedAKnownEnvironment(t *testing.T) {
	tests := []struct {
		args []string
		want string
	}{
		{[]string{"orgs"}, "no environment"},
		{[]string{"users", "-e", "dev", "org-1"}, `unknown environment "dev"`},
		{[]string{"open", "-e", "dev", "user-1"}, `unknown environment "dev"`},
	}
	for _, tt := range tests {
		root := &cobra.Command{Use: "impersonate", SilenceUsage: true}
		root.AddCommand(launcherCommands(settings{Envs: testEnvs}, nil)...)
		root.SetArgs(tt.args)
		root.SetOut(io.Discard)
		root.SetErr(io.Discard)
		if err := root.Execute(); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Errorf("%v: got %v, want %q", tt.args, err, tt.want)
		}
	}
}
