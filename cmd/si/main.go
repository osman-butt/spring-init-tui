package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"runtime/debug"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/osman-butt/spring-init-tui/internal/initializr"
	"github.com/osman-butt/spring-init-tui/internal/tui"
)

// version is set by the release build with -ldflags "-X main.version=...".
var version string

// buildVersion returns the version to report: the one the release build
// stamped in, else the module version recorded by "go install", else "dev".
func buildVersion(stamped string, readBuildInfo func() (*debug.BuildInfo, bool)) string {
	if stamped != "" {
		return stamped
	}
	if info, ok := readBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return strings.TrimPrefix(info.Main.Version, "v")
	}
	return "dev"
}

func usage() {
	fmt.Fprint(flag.CommandLine.Output(), `Usage: si [--version]

Generate a Spring Boot project with Spring Initializr, interactively.
Run it in the directory where the project should be created.

Options:
  --version   print the version and exit
  --help      show this help
`)
}

func main() {
	showVersion := flag.Bool("version", false, "print the version and exit")
	flag.Usage = usage
	flag.Parse()
	if flag.NArg() > 0 {
		fmt.Fprintf(os.Stderr, "si: unexpected argument %q\n\n", flag.Arg(0))
		flag.Usage()
		os.Exit(2)
	}
	if *showVersion {
		fmt.Println("si", buildVersion(version, debug.ReadBuildInfo))
		return
	}

	if !term.IsTerminal(os.Stdin.Fd()) || !term.IsTerminal(os.Stdout.Fd()) {
		fmt.Fprintln(os.Stderr, "si needs an interactive terminal")
		os.Exit(2)
	}

	final, err := tea.NewProgram(tui.New(initializr.NewClient())).Run()
	if err != nil {
		if errors.Is(err, tea.ErrInterrupted) {
			os.Exit(130)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if m, ok := final.(tui.Model); ok {
		switch {
		case m.Interrupted():
			os.Exit(130)
		case m.Err() != nil:
			os.Exit(1)
		}
	}
}
