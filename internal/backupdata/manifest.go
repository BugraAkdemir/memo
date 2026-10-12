// Package backupdata defines the portable user-data inventory shared by local
// export and encrypted cloud backup. Device identities and vendor OAuth files
// are deliberately outside this list.
package backupdata

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"memo/internal/imgvault"
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
	return IsImageEntry("data/"+rel) || models && strings.HasPrefix(rel, "models/")
}

// WriteSource skips only absent optional roots. Walk/open/copy errors abort the
// backup rather than producing an apparently successful incomplete archive.
func WriteSource(zw *zip.Writer, s Source) (int, error) {
	return writeSource(zw, s, nil)
}
func writeSource(zw *zip.Writer, s Source, read func(string, string) ([]byte, error)) (int, error) {
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
		if read != nil && IsImageEntry(entry) {
			var blob []byte
			blob, err = read(path, entry)
			if err == nil {
				_, err = w.Write(blob)
			}
		} else {
			_, err = io.Copy(w, f)
		}
		if err != nil {
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
func Archive(sources []Source) ([]byte, int, error) { return archive(sources, nil) }
func ArchiveImages(sources []Source, dataDir string, portable bool) ([]byte, int, error) {
	for _, dir := range []string{"images", "generated-images"} {
		sources = append(sources, Source{"data/" + dir + "/", filepath.Join(dataDir, dir)})
	}
	return archive(sources, func(path, entry string) ([]byte, error) {
		v, err := imgvault.Default()
		if err != nil {
			return nil, err
		}
		plain, err := v.ReadFile(path)
		if err != nil {
			return nil, err
		}
		if portable {
			return plain, nil
		}
		return v.SealData(plain)
	})
}
func archive(sources []Source, read func(string, string) ([]byte, error)) ([]byte, int, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	count := 0
	for _, s := range sources {
		n, err := writeSource(zw, s, read)
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

// Pictures are portable only inside an encrypted archive. The source master
// key never enters an archive; destination pictures are sealed before staging.
func IsImageEntry(name string) bool {
	return strings.HasPrefix(name, "data/images/") || strings.HasPrefix(name, "data/generated-images/")
}
func RestoreContent(name string, blob []byte, dataDir string) ([]byte, error) {
	if IsImageEntry(name) {
		v, err := imgvault.Default()
		if err != nil {
			return nil, err
		}
		plain, err := v.OpenData(blob)
		if err != nil {
			return nil, fmt.Errorf("restore image: use a password-protected portable backup: %w", err)
		}
		return v.SealData(plain)
	}
	if strings.HasPrefix(name, "sessions/") && strings.HasSuffix(name, ".json") {
		var doc any
		if err := json.Unmarshal(blob, &doc); err != nil {
			return blob, nil
		}
		var walk func(any)
		walk = func(value any) {
			switch x := value.(type) {
			case map[string]any:
				for key, v := range x {
					if key == "image_path" {
						if path, ok := v.(string); ok {
							norm := strings.ReplaceAll(path, "\\", "/")
							for _, dir := range []string{"images", "generated-images"} {
								marker := "/" + dir + "/"
								if i := strings.LastIndex(norm, marker); i >= 0 {
									rel := norm[i+1:]
									clean := filepath.Clean(filepath.FromSlash(rel))
									if !strings.Contains(clean, "..") && !filepath.IsAbs(clean) {
										x[key] = filepath.Join(dataDir, clean)
									}
									break
								}
							}
						}
					} else {
						walk(v)
					}
				}
			case []any:
				for _, v := range x {
					walk(v)
				}
			}
		}
		walk(doc)
		return json.Marshal(doc)
	}
	return blob, nil
}
