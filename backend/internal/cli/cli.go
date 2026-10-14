// Package cli parses command-line flags and env vars. (internal/config is the saved-config store.)
package cli

import "github.com/alecthomas/kong"

type CLI struct {
	Port    string           `short:"p" env:"SHOTQUEUE_PORT" default:"8080" help:"Port to serve the API and web UI on."`
	Version kong.VersionFlag `short:"v" help:"Print version and exit."`
}

// Load parses args (typically os.Args[1:]); flags override env vars, which override defaults.
func Load(args []string, version string, opts ...kong.Option) (CLI, error) {
	var c CLI
	opts = append([]kong.Option{
		kong.Name("shotqueue"),
		kong.Description("Web server to assist in automated PTZ-camera preset triggering."),
		kong.Vars{"version": version},
		kong.UsageOnError(),
	}, opts...)
	k, err := kong.New(&c, opts...)
	if err != nil {
		return c, err
	}
	_, err = k.Parse(args)
	return c, err
}
