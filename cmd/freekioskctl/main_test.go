package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompletion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"devices":{"living":{"url":"http://127.0.0.1:1","api_key":"secret"},"desk":{"url":"http://127.0.0.1:1"}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"completion", "bash"}, "bash"}, {[]string{"completion", "zsh"}, "compdef"}, {[]string{"completion", "fish"}, "complete"}, {[]string{"completion", "powershell"}, "Register-ArgumentCompleter"},
		{[]string{"--config", path, "__complete", "status", ""}, "living"},
		{[]string{"--config", path, "__complete", "status", "d"}, "desk"},
		{[]string{"__complete", "screen", "living", ""}, "off"},
		{[]string{"--config", path, "devices"}, "living"},
	} {
		t.Run(strings.Join(tc.args, " "), func(t *testing.T) {
			cmd := newCommand()
			var out bytes.Buffer
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs(tc.args)
			if err := cmd.ExecuteContext(t.Context()); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out.String(), tc.want) || strings.Contains(out.String(), "secret") {
				t.Fatal("wrong completion output")
			}
		})
	}
	for _, args := range [][]string{{"status"}, {"screen", "living"}, {"reload", "living", "extra"}, {"status", "unknown"}} {
		cmd := newCommand()
		cmd.SetArgs(append([]string{"--config", path}, args...))
		if err := cmd.ExecuteContext(t.Context()); err == nil {
			t.Fatal("invalid arguments accepted")
		}
	}
}
