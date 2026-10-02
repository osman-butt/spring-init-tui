package initializr

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const maxArchiveBytes = 64 << 20

// Request describes the project to generate. Name is also used as the
// Maven artifactId. GroupID and PackageName are optional: Initializr falls
// back to its own defaults when they are empty.
type Request struct {
	Name         string
	GroupID      string
	PackageName  string
	JavaVersion  string
	Dependencies []string
}

// PackageName returns the base Java package for a project: the group
// followed by the artifact in lower case, without the characters a package
// name cannot contain. Initializr's own default replaces those with
// underscores ("spring-demo" becomes "spring_demo"); this drops them
// ("springdemo").
func PackageName(groupID, artifactID string) string {
	var segment strings.Builder
	for _, r := range strings.ToLower(artifactID) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			segment.WriteRune(r)
		}
	}
	name := segment.String()
	switch {
	case name == "":
		return groupID
	case name[0] >= '0' && name[0] <= '9':
		name = "_" + name // an identifier cannot start with a digit
	}
	if groupID == "" {
		return name
	}
	return groupID + "." + name
}

// Generate downloads a Maven project for req and extracts it into dest,
// which must not exist yet.
func (c *Client) Generate(ctx context.Context, req Request, dest string) error {
	dest, err := filepath.Abs(dest)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(dest); err == nil {
		return fmt.Errorf("%s already exists", dest)
	}

	archive, err := c.download(ctx, req)
	if err != nil {
		return fmt.Errorf("download project: %w", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return fmt.Errorf("read project archive: %w", err)
	}
	if err := extract(zr, dest); err != nil {
		return fmt.Errorf("extract project: %w", err)
	}
	return nil
}

func (c *Client) download(ctx context.Context, req Request) ([]byte, error) {
	q := url.Values{}
	q.Set("type", "maven-project")
	q.Set("artifactId", req.Name)
	q.Set("name", req.Name)
	if req.GroupID != "" {
		q.Set("groupId", req.GroupID)
	}
	if req.PackageName != "" {
		q.Set("packageName", req.PackageName)
	}
	q.Set("javaVersion", req.JavaVersion)
	if len(req.Dependencies) > 0 {
		q.Set("dependencies", strings.Join(req.Dependencies, ","))
	}

	resp, err := c.get(ctx, c.BaseURL+"/starter.zip?"+q.Encode(), "")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	archive, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, err
	}
	if len(archive) > maxArchiveBytes {
		return nil, fmt.Errorf("archive is larger than %d bytes", maxArchiveBytes)
	}
	return archive, nil
}
