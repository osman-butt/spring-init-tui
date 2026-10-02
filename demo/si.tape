# Records demo/si.gif with VHS (https://github.com/charmbracelet/vhs).
# Run it from the repository root:
#
#   vhs demo/si.tape
#
# It uses the live start.spring.io and generates a real project in a
# temporary directory.

Output demo/si.gif

Require go

Set Shell "bash"
Set FontFamily "Hack Nerd Font"
Set FontSize 18
Set Width 1200
Set Height 700
Set Theme "Catppuccin Mocha"
Set TypingSpeed 40ms
Set WaitTimeout 30s

# Build si, put it on the PATH and work in an empty temporary directory.
Hide
Type `BIN="$(mktemp -d)" && go build -o "$BIN/si" ./cmd/si && export PATH="$BIN:$PATH" && cd "$(mktemp -d)" && clear`
Enter
Wait
Show

Type "si"
Sleep 500ms
Enter

# Project name
Wait+Screen /Project/
Sleep 1s
Type "spring-demo"
Sleep 800ms
Enter

# Group: keep the default
Wait+Screen /Group/
Sleep 1.5s
Enter

# Java version: one up from the default
Wait+Screen /Java version/
Sleep 1s
Up
Sleep 1s
Enter

# Dependencies: search, select, clear, search again
Wait+Screen /Dependencies/
Sleep 1.2s
Type "web"
Sleep 1s
Tab
Sleep 1s
Escape
Sleep 800ms
Type "jpa"
Sleep 1s
Tab
Sleep 1.5s
Enter

# Confirm
Wait+Screen /Generate spring-demo/
Sleep 2.5s
Enter

# Result
Wait+Screen /generated successfully/
Sleep 1s
