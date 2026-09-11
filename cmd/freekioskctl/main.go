package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/AnaTofuZ/freekiosk-manager/internal/config"
	"github.com/AnaTofuZ/freekiosk-manager/internal/freekiosk"
	"github.com/spf13/cobra"
)

func newCommand() *cobra.Command {
	var path string
	var timeout time.Duration
	root := &cobra.Command{Use: "freekioskctl", Short: "Manage FreeKiosk devices on your LAN", SilenceUsage: true, SilenceErrors: true}
	root.PersistentFlags().StringVar(&path, "config", config.DefaultPath(), "Configuration JSON file")
	root.PersistentFlags().DurationVar(&timeout, "timeout", 8*time.Second, "Device request timeout")
	root.AddCommand(&cobra.Command{Use: "devices", Short: "List configured devices", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		cfg, err := config.Load(path)
		if err != nil {
			return err
		}
		ids := make([]string, 0, len(cfg.Devices))
		for id := range cfg.Devices {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", id, cfg.Devices[id].Name); err != nil {
				return err
			}
		}
		return nil
	}})
	for _, spec := range []struct {
		use, short string
		min, max   int
	}{
		{"status <device>", "Get device status", 1, 1},
		{"reload <device>", "Reload the current page", 1, 1},
		{"clear-cache <device>", "Clear WebView cache, cookies and storage", 1, 1},
		{"url <device> [url]", "Get or change the current URL", 1, 2},
		{"screen <device> <on|off>", "Turn the screen on or off", 2, 2},
		{"brightness <device> <0-100>", "Change screen brightness", 2, 2},
		{"volume <device> <0-100>", "Change media volume", 2, 2},
		{"screenshot <device> <file.png>", "Save a screenshot atomically", 2, 2},
		{"toast <device> <text>", "Display a toast", 2, 2},
		{"tts <device> <text> [language]", "Speak text (optional BCP 47 language)", 2, 3},
	} {
		root.AddCommand(&cobra.Command{Use: spec.use, Short: spec.short, Args: cobra.RangeArgs(spec.min, spec.max), ValidArgsFunction: func(cmd *cobra.Command, args []string, prefix string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				cfg, err := config.Load(path)
				if err != nil {
					return nil, cobra.ShellCompDirectiveError
				}
				var ids []string
				for id := range cfg.Devices {
					if strings.HasPrefix(id, prefix) {
						ids = append(ids, id)
					}
				}
				slices.Sort(ids)
				return ids, cobra.ShellCompDirectiveNoFileComp
			}
			if len(args) == 1 && cmd.Name() == "screen" {
				return []string{"on", "off"}, cobra.ShellCompDirectiveNoFileComp
			}
			if len(args) == 1 && cmd.Name() == "screenshot" {
				return []string{"png"}, cobra.ShellCompDirectiveFilterFileExt
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		}, RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(path)
			if err != nil {
				return err
			}
			dev, ok := cfg.Devices[args[0]]
			if !ok {
				return &freekiosk.Error{Kind: "unknown_device", Message: "Unknown device"}
			}
			c, err := freekiosk.New(dev.URL, dev.APIKey, timeout)
			if err != nil {
				return err
			}
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			switch cmd.Name() {
			case "status":
				s, err := c.Status(ctx)
				if err != nil {
					return err
				}
				enc := json.NewEncoder(out)
				enc.SetIndent("", "  ")
				return enc.Encode(s)
			case "url":
				if len(args) == 1 {
					u, err := c.CurrentURL(ctx)
					if err != nil {
						return err
					}
					_, err = fmt.Fprintln(out, u)
					return err
				}
			case "screenshot":
				data, err := c.Screenshot(ctx)
				if err != nil {
					return err
				}
				return saveImage(args[1], data)
			}
			var p freekiosk.Command
			switch cmd.Name() {
			case "brightness", "volume":
				n, err := strconv.Atoi(args[1])
				if err != nil {
					return freekiosk.Invalid("Value must be an integer from 0 to 100")
				}
				p.Value = &n
			case "screen":
				p.State = args[1]
			case "url":
				p.URL = args[1]
			case "toast", "tts":
				p.Text = args[1]
				if len(args) == 3 {
					p.Language = args[2]
				}
			}
			if err := c.Execute(ctx, cmd.Name(), p); err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, "Command succeeded")
			return err
		}})
	}
	return root
}

func saveImage(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".freekiosk-*.png")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(f.Name()) }()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := newCommand().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
