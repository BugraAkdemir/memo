// SPDX-License-Identifier: AGPL-3.0-or-later

// Command memo-sign makes and checks the signed manifests the in-app updater
// trusts. It is run by the release workflows and by the maintainer when
// publishing an engine component; the private key never leaves CI / the
// maintainer's machine.
//
//	memo-sign keygen   -out KEYFILE                      (prints the public key)
//	memo-sign app      -version V4.7.0 -platform linux-amd64 -kind archive -format tar.gz \
//	                   -url URL -file PATH [-root Memo] -out DIR
//	memo-sign component -component llama -version b11456 [-requires-app 4.7.0] \
//	                   -base-url URL -out DIR -file linux/cpu=PATH[:zip] ...
//	memo-sign verify   -pub KEY FILE                     (checks FILE against FILE.sig)
//
// The signing key is read from the MEMO_UPDATE_SIGNING_KEY environment variable
// (a base64 line, as written by keygen) or from -key-file.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"memo/internal/updater"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, os.Getenv)) }

func run(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	var err error
	switch args[0] {
	case "keygen":
		err = keygen(args[1:], stdout)
	case "app":
		err = signApp(args[1:], stdout, getenv)
	case "component":
		err = signComponent(args[1:], stdout, getenv)
	case "verify":
		err = verify(args[1:], stdout)
	default:
		usage(stderr)
		return 2
	}
	if err != nil {
		fmt.Fprintln(stderr, "memo-sign:", err)
		return 1
	}
	return 0
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: memo-sign keygen|app|component|verify … (see the file's doc comment)")
}

func keygen(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("keygen", flag.ContinueOnError)
	out := fs.String("out", "", "file to write the PRIVATE key to (mode 0600)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *out == "" {
		return errors.New("-out is required")
	}
	if _, err := os.Stat(*out); err == nil {
		return fmt.Errorf("%s already exists; refusing to overwrite a signing key", *out)
	}
	pub, priv, err := updater.GenerateKey()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(*out, []byte(updater.EncodePrivate(priv)+"\n"), 0o600); err != nil {
		return err
	}
	fmt.Fprintln(stdout, updater.EncodePublic(pub))
	return nil
}

func loadKey(keyFile string, getenv func(string) string) (keyBytes []byte, err error) {
	raw := strings.TrimSpace(getenv("MEMO_UPDATE_SIGNING_KEY"))
	if raw == "" && keyFile != "" {
		b, err := os.ReadFile(keyFile)
		if err != nil {
			return nil, err
		}
		raw = strings.TrimSpace(string(b))
	}
	if raw == "" {
		return nil, errors.New("no signing key: set MEMO_UPDATE_SIGNING_KEY or pass -key-file")
	}
	return []byte(raw), nil
}

// writeSigned writes dir/name and dir/name.sig.
func writeSigned(dir, name string, body []byte, keyRaw []byte) (string, error) {
	priv, err := updater.DecodePrivate(string(keyRaw))
	if err != nil {
		return "", err
	}
	sig, err := updater.Sign(priv, body)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, body, 0o644); err != nil {
		return "", err
	}
	if err := os.WriteFile(path+".sig", []byte(sig+"\n"), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

func signApp(args []string, stdout io.Writer, getenv func(string) string) error {
	fs := flag.NewFlagSet("app", flag.ContinueOnError)
	var (
		version  = fs.String("version", "", "release version, V4.7.0")
		platform = fs.String("platform", "", "linux-amd64, darwin-arm64, windows-amd64, …")
		kind     = fs.String("kind", updater.KindArchive, "archive | installer")
		format   = fs.String("format", "tar.gz", "tar.gz | zip | exe")
		url      = fs.String("url", "", "where the file is published")
		file     = fs.String("file", "", "the local file (its size and SHA-256 go into the manifest)")
		root     = fs.String("root", "", "top-level directory inside the archive")
		out      = fs.String("out", "", "directory to write <platform>.json and .json.sig into")
		keyFile  = fs.String("key-file", "", "file holding the private key (instead of the environment)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	for name, v := range map[string]string{"-version": *version, "-platform": *platform, "-url": *url, "-file": *file, "-out": *out} {
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	key, err := loadKey(*keyFile, getenv)
	if err != nil {
		return err
	}
	st, err := os.Stat(*file)
	if err != nil {
		return err
	}
	sum, err := updater.SHA256File(*file)
	if err != nil {
		return err
	}
	m := updater.AppManifest{
		Schema: updater.SchemaVersion, Type: updater.TypeApp, Version: *version, Platform: *platform,
		Kind: *kind, Format: *format, URL: *url, SHA256: sum, Size: st.Size(), Root: *root,
		Released: time.Now().UTC().Format("2006-01-02"),
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	// Never publish something the app would refuse to read.
	if _, err := updater.ParseAppManifest(body); err != nil {
		return fmt.Errorf("the manifest would be rejected by the app: %w", err)
	}
	path, err := writeSigned(*out, *platform+".json", body, key)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, path)
	return nil
}

type fileFlags []string

func (f *fileFlags) String() string     { return strings.Join(*f, ",") }
func (f *fileFlags) Set(s string) error { *f = append(*f, s); return nil }

func signComponent(args []string, stdout io.Writer, getenv func(string) string) error {
	fs := flag.NewFlagSet("component", flag.ContinueOnError)
	var (
		component   = fs.String("component", "", "llama | whisper | cliproxy")
		version     = fs.String("version", "", "release tag, b11456 / 1.8.6 / v8.0.26")
		requiresApp = fs.String("requires-app", "", "oldest app version that can drive it, 4.7.0")
		baseURL     = fs.String("base-url", "", "directory URL the archives are published under")
		out         = fs.String("out", "", "directory to write manifest.json and manifest.json.sig into")
		keyFile     = fs.String("key-file", "", "file holding the private key (instead of the environment)")
		files       fileFlags
	)
	fs.Var(&files, "file", "OS/FOLDER=PATH[:zip] — one engine folder archive (repeatable)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	for name, v := range map[string]string{"-component": *component, "-version": *version, "-base-url": *baseURL, "-out": *out} {
		if v == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	if len(files) == 0 {
		return errors.New("at least one -file is required")
	}
	key, err := loadKey(*keyFile, getenv)
	if err != nil {
		return err
	}
	m := updater.ComponentManifest{
		Schema: updater.SchemaVersion, Type: updater.TypeComponent, Component: *component,
		Version: *version, RequiresApp: *requiresApp, Files: map[string]updater.File{},
	}
	for _, spec := range files {
		folder, rest, ok := strings.Cut(spec, "=")
		if !ok {
			return fmt.Errorf("-file %q: want OS/FOLDER=PATH", spec)
		}
		path, format := rest, "tar.gz"
		if strings.HasSuffix(rest, ":zip") {
			path, format = strings.TrimSuffix(rest, ":zip"), "zip"
		}
		st, err := os.Stat(path)
		if err != nil {
			return err
		}
		sum, err := updater.SHA256File(path)
		if err != nil {
			return err
		}
		m.Files[folder] = updater.File{
			URL:    strings.TrimRight(*baseURL, "/") + "/" + filepath.Base(path),
			SHA256: sum, Size: st.Size(), Format: format,
		}
	}
	body, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	body = append(body, '\n')
	if _, err := updater.ParseComponentManifest(body); err != nil {
		return fmt.Errorf("the manifest would be rejected by the app: %w", err)
	}
	path, err := writeSigned(*out, "manifest.json", body, key)
	if err != nil {
		return err
	}
	fmt.Fprintln(stdout, path)
	return nil
}

func verify(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("verify", flag.ContinueOnError)
	pub := fs.String("pub", "", "the public key, base64")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *pub == "" || fs.NArg() != 1 {
		return errors.New("usage: memo-sign verify -pub KEY FILE")
	}
	key, err := updater.DecodePublic(*pub)
	if err != nil {
		return err
	}
	body, err := os.ReadFile(fs.Arg(0))
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(fs.Arg(0) + ".sig")
	if err != nil {
		return err
	}
	if err := updater.Verify(key, body, string(sig)); err != nil {
		return err
	}
	fmt.Fprintln(stdout, "signature OK:", fs.Arg(0))
	return nil
}
