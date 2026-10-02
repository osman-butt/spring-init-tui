package initializr

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

const maxExtractedBytes = 256 << 20

// extract unpacks zr into dest. It works in a temporary directory next to
// dest and renames it into place at the end, so a failed or interrupted run
// leaves no half-written project behind.
func extract(zr *zip.Reader, dest string) (err error) {
	tmp, err := os.MkdirTemp(filepath.Dir(dest), ".spring-init-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			os.RemoveAll(tmp)
		}
	}()

	remaining := int64(maxExtractedBytes)
	for _, f := range zr.File {
		// Reject entries that would land outside tmp ("../x", "/etc/x").
		if !filepath.IsLocal(f.Name) {
			return fmt.Errorf("illegal path in archive: %q", f.Name)
		}
		path := filepath.Join(tmp, f.Name)

		switch mode := f.Mode(); {
		case mode.IsDir():
			if err := os.MkdirAll(path, 0o755); err != nil {
				return err
			}
		case mode.IsRegular():
			n, err := extractFile(f, path, remaining)
			if err != nil {
				return err
			}
			remaining -= n
		default:
			return fmt.Errorf("unsupported file type in archive: %q", f.Name)
		}
	}

	// MkdirTemp creates the directory with 0700.
	if err := os.Chmod(tmp, 0o755); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

// extractFile writes f to path and returns the number of bytes written. It
// fails if the file is larger than limit.
func extractFile(f *zip.File, path string, limit int64) (int64, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return 0, err
	}

	// Keep the executable bit of wrapper scripts such as mvnw.
	perm := f.Mode().Perm()
	if perm == 0 {
		perm = 0o644
	}

	src, err := f.Open()
	if err != nil {
		return 0, err
	}
	defer src.Close()

	dst, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return 0, err
	}

	n, err := io.Copy(dst, io.LimitReader(src, limit+1))
	if closeErr := dst.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return n, err
	}
	if n > limit {
		return n, fmt.Errorf("archive expands to more than %d bytes", maxExtractedBytes)
	}
	return n, nil
}
