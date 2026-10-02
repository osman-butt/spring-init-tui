# spring-init-tui

A terminal UI for [Spring Initializr](https://start.spring.io), built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Pick a project name, a group, a Java version and dependencies, and it generates a Maven project in a new directory.

The project name is also the Maven artifact. The Java package is the group plus the name without `-`, `_` and `.`, so `spring-demo` in group `com.example` becomes `com.example.springdemo`.

## Install

The command is called `si`.

### Download a release

Download the archive for your system from the [releases page](https://github.com/osman-butt/spring-init-tui/releases). There are builds for Linux, macOS and Windows, each for `amd64` and `arm64`, named `spring-init-tui_<version>_<os>_<arch>`.

Unpack it and put `si` on your `PATH`, for example on macOS with Apple silicon:

```sh
tar -xzf spring-init-tui_0.1.0_darwin_arm64.tar.gz si
sudo mv si /usr/local/bin/
si --version
```

To verify a download, get `checksums.txt` from the same release and run `shasum -a 256 -c checksums.txt --ignore-missing` (`sha256sum` on Linux).

- **macOS:** if you downloaded the archive with a browser, macOS blocks the unsigned binary. Remove the quarantine flag with `xattr -d com.apple.quarantine si`.
- **Windows PowerShell:** `si` is a built-in alias for `Set-Item`, so run `si.exe`.

### With Go

Requires Go 1.26 or newer.

```sh
go install github.com/osman-butt/spring-init-tui/cmd/si@latest
```

## Usage

Run it in the directory where the project should be created:

```sh
si
```

Then start the generated project:

```sh
cd <project-name>
./mvnw spring-boot:run
```

On Windows the last command is `.\mvnw.cmd spring-boot:run`.

## Keys

| Key | Action |
| --- | --- |
| `↑` / `↓` | Move |
| `enter` | Continue |
| `esc` | Go back |
| `ctrl+c` | Quit |

On the dependency screen, just type to search:

| Key | Action |
| --- | --- |
| any text | Search by name or ID |
| `tab` | Select or deselect the highlighted dependency |
| `pgup` / `pgdn` | Previous or next page |
| `esc` | Clear the search, or go back when it is empty |

## Development

```sh
go run ./cmd/si
go test ./...
```
