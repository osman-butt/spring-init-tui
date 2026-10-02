# SI 

[![CI](https://github.com/osman-butt/spring-init-tui/actions/workflows/ci.yml/badge.svg)](https://github.com/osman-butt/spring-init-tui/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/osman-butt/spring-init-tui)](https://github.com/osman-butt/spring-init-tui/releases/latest)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue)](LICENSE)

`si` is a terminal UI for [Spring Initializr](https://start.spring.io). Choose an artifact, group, package name, Spring Boot version, Java version and dependencies, and it generates a ready-to-run Maven project.

![Generating a Spring Boot project with si](demo/si.gif)

## Install

With [Homebrew](https://brew.sh) on macOS or Linux:

```sh
brew install osman-butt/tap/si
```

With Go 1.26 or newer:

```sh
go install github.com/osman-butt/spring-init-tui/cmd/si@latest
```

Or download the archive for your operating system and CPU from the [latest release](https://github.com/osman-butt/spring-init-tui/releases/latest), unpack it, and move `si` to a directory on your `PATH`.

- **macOS:** a binary downloaded with a browser is blocked because it is not signed. Run `xattr -d com.apple.quarantine si` once to allow it. Homebrew does this for you.
- **Windows PowerShell:** `si` is a built-in alias for `Set-Item`, so run `si.exe`.

## Usage

Run `si` in the directory where the project should be created and follow the prompts. The keys for each step are shown at the bottom of the screen.

## License

[MIT](LICENSE). `si` is an independent project and is not affiliated with Spring.
