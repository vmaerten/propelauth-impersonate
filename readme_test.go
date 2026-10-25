package main

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// The README is the reference of the flags, settings and subcommands: a new one fails
// here until it is documented.
func TestReadmeDocumentsEveryOption(t *testing.T) {
	data, err := os.ReadFile("README.md")
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	rootCommand(settings{}, nil).Flags().VisitAll(func(f *pflag.Flag) {
		want = append(want, "`--"+f.Name+"`")
		if f.Shorthand != "" {
			want = append(want, "`-"+f.Shorthand+"`")
		}
	})
	for _, key := range scalarKeys {
		want = append(want, "`"+key+"`")
	}
	env := reflect.TypeFor[environment]()
	for i := range env.NumField() {
		want = append(want, "`"+env.Field(i).Tag.Get("toml")+"`")
	}
	for _, cmd := range configCommand("").Commands() {
		want = append(want, "`impersonate config "+cmd.Name())
	}
	for _, cmd := range launcherCommands(settings{}, nil) {
		want = append(want, "`impersonate "+cmd.Name())
		cmd.Flags().VisitAll(func(f *pflag.Flag) { want = append(want, "--"+f.Name+"`") })
	}
	for _, s := range want {
		if !strings.Contains(string(data), s) {
			t.Errorf("README.md does not mention %s", s)
		}
	}
}
