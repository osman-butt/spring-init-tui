# spring-init-tui

A terminal UI for [Spring Initializr](https://start.spring.io), built with Go and [Bubble Tea](https://github.com/charmbracelet/bubbletea).

Pick a project name, a group, a Java version and dependencies, and it generates a Maven project in a new directory.

The project name is also the Maven artifact. The Java package is the group plus the name without `-`, `_` and `.`, so `spring-demo` in group `com.example` becomes `com.example.springdemo`.

## Install

Requires Go 1.26 or newer.

```sh
go install github.com/osman-butt/spring-init-tui@latest
```

## Usage

Run it in the directory where the project should be created:

```sh
spring-init-tui
```

Then start the generated project:

```sh
cd <project-name>
./mvnw spring-boot:run
```

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
go run .
go test ./...
```
