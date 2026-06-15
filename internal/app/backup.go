package app

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"memo/internal/config"
	"memo/internal/sessions"
)

// ExportData packages all user data (except models) into a .memo zip archive.
func (a *App) ExportData(includeModels bool) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	addFile := func(name, src string) error {
		fi, err := os.Stat(src)
		if err != nil {
			return nil // skip missing files
		}
		if fi.IsDir() {
			return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return err
				}
				rel, _ := filepath.Rel(src, path)
				w, err := zw.Create(filepath.Join(name, rel))
				if err != nil {
					return err
				}
				f, err := os.Open(path)
				if err != nil {
					return err
				}
				_, err = io.Copy(w, f)
				f.Close()
				return err
			})
		}
		w, err := zw.Create(name)
		if err != nil {
			return err
		}
		f, err := os.Open(src)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		f.Close()
		return err
	}

	addFile("sessions/", config.DataPath("sessions"))
	addFile("config/config.yaml", "config/config.yaml")
	addFile("data/providers.json", config.DataPath("providers.json"))
	addFile("data/orchestra.json", config.DataPath("orchestra.json"))
	addFile("data/memory/", config.DataPath("memory"))
	addFile("data/whatsapp/", config.DataPath("whatsapp"))
	if includeModels {
		addFile("data/models/", config.DataPath("models"))
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ImportData restores user data from a .memo zip archive.
func (a *App) ImportData(data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("import: invalid zip: %w", err)
	}

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		clean := filepath.Clean(filepath.FromSlash(f.Name))
		if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || filepath.IsAbs(clean) {
			continue
		}

		var target string
		switch {
		case strings.HasPrefix(f.Name, "sessions/"):
			target = filepath.Join(config.DataDir(), "sessions", strings.TrimPrefix(f.Name, "sessions/"))
		case strings.HasPrefix(f.Name, "data/"):
			target = config.DataPath(filepath.FromSlash(strings.TrimPrefix(f.Name, "data/")))
		default:
			target = clean
		}

		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("import: mkdir: %w", err)
		}

		rc, err := f.Open()
		if err != nil {
			return fmt.Errorf("import: open %s: %w", f.Name, err)
		}

		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return fmt.Errorf("import: create %s: %w", target, err)
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			return fmt.Errorf("import: write %s: %w", target, err)
		}
	}

	// Re-initialize components after import
	a.sessionsMu.Lock()
	if a.sessions != nil {
		sm, err := sessions.NewManager(config.DataPath("sessions"))
		if err == nil {
			a.sessions = sm
		}
	}
	a.sessionsMu.Unlock()

	return nil
}

// WipeAllData removes all user data: sessions, memory, whatsapp, providers.
func (a *App) WipeAllData() error {
	dirs := []string{
		config.DataPath("sessions"),
		config.DataPath("memory"),
		config.DataPath("whatsapp"),
		config.DataPath("providers.json"),
		config.DataPath("orchestra.json"),
	}
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			return fmt.Errorf("wipe: %s: %w", d, err)
		}
	}

	a.sessionsMu.Lock()
	if a.sessions != nil {
		sm, err := sessions.NewManager(config.DataPath("sessions"))
		if err == nil {
			a.sessions = sm
		}
	}
	a.sessionsMu.Unlock()

	a.storeMu.Lock()
	a.store = nil
	a.storeMu.Unlock()

	log.Println("All user data wiped")
	return nil
}
