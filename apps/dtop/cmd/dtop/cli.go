package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildDate = "unknown"
)

type cliOptions struct {
	configPath string
	version    bool
}

func parseCLI(args []string, output io.Writer) (cliOptions, error) {
	var options cliOptions
	flags := flag.NewFlagSet("dtop", flag.ContinueOnError)
	flags.SetOutput(output)
	flags.StringVar(&options.configPath, "config", "", "load an additional configuration file with highest priority")
	flags.BoolVar(&options.version, "version", false, "print version information and exit")
	flags.Usage = func() {
		fmt.Fprintln(output, "Usage: dtop [--config PATH] [--version]")
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Monitor and operate a local Docker Engine and registered Compose projects.")
		fmt.Fprintln(output)
		fmt.Fprintln(output, "Options:")
		fmt.Fprintln(output, "  --config PATH  load an additional configuration file with highest priority")
		fmt.Fprintln(output, "  --version      print version information and exit")
		fmt.Fprintln(output, "  --help         print this help and exit")
	}
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	configSet := false
	flags.Visit(func(option *flag.Flag) {
		configSet = configSet || option.Name == "config"
	})
	if configSet && options.configPath == "" {
		return cliOptions{}, errors.New("--config requires a nonempty path")
	}
	if flags.NArg() != 0 {
		return cliOptions{}, fmt.Errorf("unexpected argument %q", flags.Arg(0))
	}
	return options, nil
}

func isHelp(err error) bool {
	return errors.Is(err, flag.ErrHelp)
}

func versionText() string {
	return fmt.Sprintf("dtop %s (commit %s, built %s)", version, commit, buildDate)
}
