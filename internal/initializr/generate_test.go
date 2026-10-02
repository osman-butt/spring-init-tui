package initializr

import (
	"archive/zip"
	"bytes"
	"context"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type zipEntry struct {
	name string
	mode fs.FileMode // 0 stores no mode bits
	body string
}

func buildZip(t *testing.T, entries []zipEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		fh := &zip.FileHeader{Name: e.name, Method: zip.Deflate}
		if e.mode != 0 {
			fh.SetMode(e.mode)
		}
		w, err := zw.CreateHeader(fh)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(e.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func projectZip(t *testing.T) []byte {
	return buildZip(t, []zipEntry{
		{name: "src/", mode: fs.ModeDir | 0o755},
		{name: "src/main/java/com/example/demo/DemoApplication.java", mode: 0o644, body: "class DemoApplication {}"},
		{name: "pom.xml", mode: 0o644, body: "<project/>"},
		{name: "mvnw", mode: 0o755, body: "#!/bin/sh"},
		{name: "HELP.md", body: "# Help"},
	})
}

func serveZip(archive []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		w.Write(archive)
	}
}

// assertNoLeftovers checks that dir holds only the wanted entries, i.e. no
// temporary extraction directory was left behind.
func assertNoLeftovers(t *testing.T, dir string, want ...string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("entries in %s = %v, want %v", dir, got, want)
	}
}

func assertMode(t *testing.T, path string, want fs.FileMode) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Errorf("mode of %s = %o, want %o", filepath.Base(path), got, want)
	}
}

func TestGenerate(t *testing.T) {
	var gotPath string
	var gotQuery url.Values
	archive := projectZip(t)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.Query()
		serveZip(archive)(w, r)
	})

	parent := t.TempDir()
	dest := filepath.Join(parent, "demo")
	req := Request{
		Name:         "spring-demo",
		GroupID:      "dev.osman",
		PackageName:  "dev.osman.springdemo",
		BootVersion:  "4.0.8",
		JavaVersion:  "21",
		Dependencies: []string{"web", "data-jpa"},
	}

	if err := c.Generate(context.Background(), req, dest); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if gotPath != "/starter.zip" {
		t.Errorf("path = %q, want /starter.zip", gotPath)
	}
	wantQuery := map[string]string{
		"type":         "maven-project",
		"artifactId":   "spring-demo",
		"name":         "spring-demo",
		"groupId":      "dev.osman",
		"packageName":  "dev.osman.springdemo",
		"bootVersion":  "4.0.8",
		"javaVersion":  "21",
		"dependencies": "web,data-jpa",
	}
	for k, want := range wantQuery {
		if got := gotQuery.Get(k); got != want {
			t.Errorf("query %s = %q, want %q", k, got, want)
		}
	}

	body, err := os.ReadFile(filepath.Join(dest, "src/main/java/com/example/demo/DemoApplication.java"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "class DemoApplication {}" {
		t.Errorf("DemoApplication.java = %q", body)
	}

	assertMode(t, dest, 0o755)
	assertMode(t, filepath.Join(dest, "mvnw"), 0o755)
	assertMode(t, filepath.Join(dest, "pom.xml"), 0o644)
	assertMode(t, filepath.Join(dest, "HELP.md"), 0o644)
	assertNoLeftovers(t, parent, "demo")
}

func TestGenerateLeavesOutEmptyOptions(t *testing.T) {
	var gotQuery url.Values
	archive := projectZip(t)
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		serveZip(archive)(w, r)
	})

	dest := filepath.Join(t.TempDir(), "demo")
	if err := c.Generate(context.Background(), Request{Name: "demo", JavaVersion: "17"}, dest); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	for _, param := range []string{"groupId", "packageName", "bootVersion", "dependencies"} {
		if gotQuery.Has(param) {
			t.Errorf("%s = %q, want the parameter to be absent", param, gotQuery.Get(param))
		}
	}
}

func TestPackageName(t *testing.T) {
	tests := []struct {
		groupID, artifactID, want string
	}{
		{"com.example", "demo", "com.example.demo"},
		{"com.example", "spring-demo", "com.example.springdemo"},
		{"com.example", "my_app.v2", "com.example.myappv2"},
		{"dev.osman", "MyApp", "dev.osman.myapp"},
		{"com.example", "2048-game", "com.example._2048game"},
		{"com.example", "---", "com.example"},
		{"", "spring-demo", "springdemo"},
	}
	for _, tt := range tests {
		if got := PackageName(tt.groupID, tt.artifactID); got != tt.want {
			t.Errorf("PackageName(%q, %q) = %q, want %q", tt.groupID, tt.artifactID, got, tt.want)
		}
	}
}

func TestGenerateRelativeDest(t *testing.T) {
	c := newTestClient(t, serveZip(projectZip(t)))

	parent := t.TempDir()
	t.Chdir(parent)

	if err := c.Generate(context.Background(), Request{Name: "demo", JavaVersion: "17"}, "demo"); err != nil {
		t.Fatalf("Generate() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "demo", "pom.xml")); err != nil {
		t.Error(err)
	}
	assertNoLeftovers(t, parent, "demo")
}

func TestGenerateExistingDest(t *testing.T) {
	requested := false
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		requested = true
	})

	parent := t.TempDir()
	dest := filepath.Join(parent, "demo")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatal(err)
	}

	err := c.Generate(context.Background(), Request{Name: "demo", JavaVersion: "17"}, dest)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("Generate() error = %v, want it to contain %q", err, "already exists")
	}
	if requested {
		t.Error("Generate() contacted the server although dest exists")
	}
	assertNoLeftovers(t, dest)
}

func TestGenerateErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "bad request with message",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(`{"status":400,"message":"Unknown dependency 'nope' check project metadata"}`))
			},
			wantErr: "download project: unexpected status 400 Bad Request: Unknown dependency 'nope'",
		},
		{
			name: "not a zip",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("definitely not a zip archive"))
			},
			wantErr: "read project archive:",
		},
		{
			name:    "path traversal",
			handler: serveZip(buildZip(t, []zipEntry{{name: "pom.xml", body: "<project/>"}, {name: "../evil.txt", body: "x"}})),
			wantErr: `illegal path in archive: "../evil.txt"`,
		},
		{
			name:    "absolute path",
			handler: serveZip(buildZip(t, []zipEntry{{name: "/tmp/evil.txt", body: "x"}})),
			wantErr: `illegal path in archive: "/tmp/evil.txt"`,
		},
		{
			name:    "symlink",
			handler: serveZip(buildZip(t, []zipEntry{{name: "link", mode: fs.ModeSymlink | 0o777, body: "/etc/passwd"}})),
			wantErr: `unsupported file type in archive: "link"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, tt.handler)

			// Nest dest one level so "../evil.txt" would land in a
			// directory the test owns.
			root := t.TempDir()
			parent := filepath.Join(root, "work")
			if err := os.Mkdir(parent, 0o755); err != nil {
				t.Fatal(err)
			}

			err := c.Generate(context.Background(), Request{Name: "demo", JavaVersion: "17"}, filepath.Join(parent, "demo"))
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Generate() error = %v, want it to contain %q", err, tt.wantErr)
			}
			assertNoLeftovers(t, parent)
			assertNoLeftovers(t, root, "work")
		})
	}
}
