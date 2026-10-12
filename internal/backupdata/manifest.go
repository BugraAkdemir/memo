// Package backupdata defines the portable user-data inventory shared by local
// export and encrypted cloud backup. Device identities and vendor OAuth files
// are deliberately outside this list.
package backupdata

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Source struct{ Entry, Path string }

var RootFiles = []string{"providers.json", "orchestra.json", "permissions.json", "machine.key", "session.key", "telegram.json", "active_skills.json", "skills_imported.json", "livemode_engines.json", "stt_providers.json", "tts_providers.json"}
var DataDirectories = []string{"memory", "whatsapp", "mood", "calendar", "profile", "routines", "tasklists", "stats", "agent-backups", "skills"}

func Sources(dataDir, memoryDir, configFile string, cloudLayout, models bool) []Source {
	out := []Source{{"sessions/", filepath.Join(dataDir, "sessions")}, {"config/config.yaml", configFile}}
	for _, name := range RootFiles {
		out = append(out, Source{"data/" + name, filepath.Join(dataDir, name)})
	}
	for _, name := range DataDirectories {
		path := filepath.Join(dataDir, name)
		entry := "data/" + name + "/"
		if name == "memory" {
			path = memoryDir
		}
		if cloudLayout && (name == "memory" || name == "mood") {
			entry = name + "/"
		}
		out = append(out, Source{entry, path})
	}
	if models {
		out = append(out, Source{"data/models/", filepath.Join(dataDir, "models")})
	}
	return out
}
func IsDataEntry(rel string, models bool) bool {
	for _, name := range RootFiles {
		if rel == name {
			return true
		}
	}
	for _, name := range DataDirectories {
		if strings.HasPrefix(rel, name+"/") {
			return true
		}
	}
	return models && strings.HasPrefix(rel, "models/")
}

// WriteSource skips only absent optional roots. Walk/open/copy errors abort the
// backup rather than producing an apparently successful incomplete archive.
func WriteSource(zw *zip.Writer, s Source) (int, error) {
	if s.Path == "" {
		return 0, nil
	}
	fi, err := os.Lstat(s.Path)
	if os.IsNotExist(err) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	count := 0
	add := func(path, entry string, info os.FileInfo) error {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("backup: symbolic link cannot be archived: %s", path)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("backup: not a regular file: %s", path)
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		header, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(entry)
		header.Method = zip.Deflate
		w, err := zw.CreateHeader(header)
		if err != nil {
			return err
		}
		if _, err = io.Copy(w, f); err != nil {
			return err
		}
		count++
		return nil
	}
	if fi.IsDir() {
		err = filepath.Walk(s.Path, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if strings.HasSuffix(path, ".tmp") || strings.HasSuffix(path, ".synctmp") {
				return nil
			}
			rel, err := filepath.Rel(s.Path, path)
			if err != nil {
				return err
			}
			return add(path, strings.TrimSuffix(s.Entry, "/")+"/"+filepath.ToSlash(rel), info)
		})
	} else {
		err = add(s.Path, s.Entry, fi)
	}
	if err != nil {
		return count, fmt.Errorf("backup %s: %w", s.Entry, err)
	}
	return count, nil
}
func Archive(sources []Source) ([]byte, int, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	count := 0
	for _, s := range sources {
		n, err := WriteSource(zw, s)
		if err != nil {
			zw.Close()
			return nil, 0, err
		}
		count += n
	}
	if err := zw.Close(); err != nil {
		return nil, 0, err
	}
	return buf.Bytes(), count, nil
}
