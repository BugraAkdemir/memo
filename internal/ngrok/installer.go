package ngrok

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"memo/internal/logx"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const cdnPrefix = "https://bin.ngrok.com/c/bNyj1mQVY4c/"

var downloadURLs = map[string]string{
	"linux/amd64":   cdnPrefix + "ngrok-v3-stable-linux-amd64.tgz",
	"linux/arm64":   cdnPrefix + "ngrok-v3-stable-linux-arm64.tgz",
	"darwin/amd64":  cdnPrefix + "ngrok-v3-stable-darwin-amd64.zip",
	"darwin/arm64":  cdnPrefix + "ngrok-v3-stable-darwin-arm64.zip",
	"windows/amd64": cdnPrefix + "ngrok-v3-stable-windows-amd64.zip",
}

func binaryName() string {
	if runtime.GOOS == "windows" {
		return "ngrok.exe"
	}
	return "ngrok"
}

func Install(baseDir string) (string, error) {
	binDir := filepath.Join(baseDir, "binaries", runtime.GOOS)
	binPath := filepath.Join(binDir, binaryName())

	if _, err := os.Stat(binPath); err == nil {
		logx.Printf("[ngrok] Binary exists: %s", binPath)
		return binPath, nil
	}

	// Check bundled location (project root binaries/)
	bundledPath := filepath.Join(".", "binaries", runtime.GOOS, binaryName())
	if _, err := os.Stat(bundledPath); err == nil {
		logx.Printf("[ngrok] Using bundled binary: %s", bundledPath)
		if err := os.MkdirAll(binDir, 0755); err == nil {
			os.Remove(binPath)
		}
		return bundledPath, nil
	}

	key := runtime.GOOS + "/" + runtime.GOARCH
	url, ok := downloadURLs[key]
	if !ok {
		return "", fmt.Errorf("unsupported platform: %s", key)
	}

	logx.Printf("[ngrok] Downloading from %s", url)

	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download: HTTP %d", resp.StatusCode)
	}

	if err := os.MkdirAll(binDir, 0755); err != nil {
		return "", fmt.Errorf("mkdir: %w", err)
	}

	if strings.HasSuffix(url, ".tgz") {
		if err := extractTGZ(resp.Body, binDir); err != nil {
			return "", fmt.Errorf("extract: %w", err)
		}
	} else if strings.HasSuffix(url, ".zip") {
		if err := extractZIP(resp.Body, binDir); err != nil {
			return "", fmt.Errorf("extract: %w", err)
		}
	}

	if runtime.GOOS != "windows" {
		os.Chmod(binPath, 0755)
	}

	logx.Printf("[ngrok] Installed to %s", binPath)
	return binPath, nil
}

func extractTGZ(r io.Reader, destDir string) error {
	gzr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(h.Name, "./")
		if name == "ngrok" || name == "ngrok.exe" {
			out := filepath.Join(destDir, name)
			f, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(h.Mode))
			if err != nil {
				return err
			}
			defer f.Close()
			_, err = io.Copy(f, tr)
			return err
		}
	}
	return fmt.Errorf("binary not found in archive")
}

func extractZIP(r io.Reader, destDir string) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}

	for _, f := range zr.File {
		if f.Name == "ngrok" || f.Name == "ngrok.exe" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			out := filepath.Join(destDir, f.Name)
			wr, err := os.OpenFile(out, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
			if err != nil {
				return err
			}
			defer wr.Close()
			_, err = io.Copy(wr, rc)
			return err
		}
	}
	return fmt.Errorf("binary not found in archive")
}
