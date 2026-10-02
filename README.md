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
| `↑` / `↓` or `k` / `j` | Move |
| `space` | Select a dependency |
| `/` | Filter dependencies |
| `enter` | Continue |
| `esc` | Go back |
| `q` or `ctrl+c` | Quit |

## Development

```sh
go run .
go test ./...
```
