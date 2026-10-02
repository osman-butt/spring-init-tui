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

// Request describes the project to generate.
type Request struct {
	Name         string
	JavaVersion  string
	Dependencies []string
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
