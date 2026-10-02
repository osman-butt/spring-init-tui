package initializr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

func newTestClient(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &Client{BaseURL: srv.URL, HTTP: srv.Client()}
}

func TestMetadata(t *testing.T) {
	fixture, err := os.ReadFile("testdata/metadata.json")
	if err != nil {
		t.Fatal(err)
	}

	var gotAccept, gotUserAgent string
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		gotAccept = r.Header.Get("Accept")
		gotUserAgent = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", metadataMediaType)
		w.Write(fixture)
	})

	md, err := c.Metadata(context.Background())
	if err != nil {
		t.Fatalf("Metadata() error = %v", err)
	}

	if gotAccept != metadataMediaType {
		t.Errorf("Accept header = %q, want %q", gotAccept, metadataMediaType)
	}
	if gotUserAgent != userAgent {
		t.Errorf("User-Agent header = %q, want %q", gotUserAgent, userAgent)
	}

	wantBoot := []BootVersion{
		{ID: "4.2.0-SNAPSHOT", Name: "4.2.0 (SNAPSHOT)"},
		{ID: "4.2.0-M2", Name: "4.2.0 (M2)"},
		{ID: "4.1.1", Name: "4.1.1"},
		{ID: "4.0.8", Name: "4.0.8"},
	}
	if !slices.Equal(md.BootVersions, wantBoot) {
		t.Errorf("BootVersions = %v, want %v", md.BootVersions, wantBoot)
	}
	if md.DefaultBootVersion != "4.1.1" {
		t.Errorf("DefaultBootVersion = %q, want %q", md.DefaultBootVersion, "4.1.1")
	}
	if want := []string{"27", "25", "21", "17"}; !slices.Equal(md.JavaVersions, want) {
		t.Errorf("JavaVersions = %v, want %v", md.JavaVersions, want)
	}
	if md.DefaultGroupID != "com.example" {
		t.Errorf("DefaultGroupID = %q, want %q", md.DefaultGroupID, "com.example")
	}
	if md.DefaultJavaVersion != "17" {
		t.Errorf("DefaultJavaVersion = %q, want %q", md.DefaultJavaVersion, "17")
	}

	if len(md.Dependencies) != 4 {
		t.Fatalf("len(Dependencies) = %d, want 4", len(md.Dependencies))
	}
	web := md.Dependencies[2]
	if web.ID != "web" || web.Name != "Spring Web" || web.Group != "Web" {
		t.Errorf("Dependencies[2] = %+v, want web / Spring Web / Web", web)
	}
	if !strings.HasPrefix(web.Description, "Build web") {
		t.Errorf("Dependencies[2].Description = %q", web.Description)
	}
}

func TestMetadataErrors(t *testing.T) {
	tests := []struct {
		name    string
		handler http.HandlerFunc
		wantErr string
	}{
		{
			name: "server error",
			handler: func(w http.ResponseWriter, r *http.Request) {
				http.Error(w, "boom", http.StatusInternalServerError)
			},
			wantErr: "fetch metadata: unexpected status 500",
		},
		{
			name: "invalid json",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Write([]byte("<html>not json</html>"))
			},
			wantErr: "decode metadata:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestClient(t, tt.handler)
			_, err := c.Metadata(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Metadata() error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestMetadataCancelled(t *testing.T) {
	c := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := c.Metadata(ctx); err == nil {
		t.Error("Metadata() with cancelled context returned no error")
	}
}
