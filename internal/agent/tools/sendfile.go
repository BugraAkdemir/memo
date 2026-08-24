package tools

import (
	"archive/zip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxShareFileBytes caps what share_file will actually hand off to a
// channel (WhatsApp upload, Telegram bot upload, or the frontend download
// link) — comfortably under Telegram Bot API's own 50MB upload ceiling,
// the tightest of the three destinations, applied uniformly rather than
// branching the limit per channel.
const maxShareFileBytes = 45 * 1024 * 1024

// FileSender is the interface share_file uses to actually deliver a file —
// set by App after initialization. Unlike WhatsAppClient.SendMessage above,
// DeliverFile has no destination parameter of any kind: exactly like
// Routines (see routine.go's doc comment for the full reasoning), the
// delivery target is resolved internally from ctx — which self-chat
// surface, if any, originated this call — never supplied by the model.
//
// isTempFile tells the implementation whether fullPath is a temp file
// share_file created (a zip it's fine to delete once nobody needs it
// anymore) or the user's own real file (must never be deleted, even much
// later — e.g. once an outbox download link expires).
//
// consumed reports whether the caller (ShareFile) may delete fullPath
// immediately once DeliverFile returns — true whenever DeliverFile is done
// touching the path right now, whether the send succeeded or failed (there
// is no retry path, so a failed WhatsApp/Telegram attempt has no more use
// for the file either); false only when the implementation is still
// holding onto fullPath for later (the frontend download-link case), where
// the caller must leave it alone and rely on the implementation's own
// later cleanup instead.
var FileSender interface {
	DeliverFile(ctx context.Context, fullPath, displayName string, isTempFile bool) (message string, consumed bool, err error)
}

// ShareFileArgs is share_file's only argument — see the FileSender doc
// comment above for why there is no destination/channel parameter.
type ShareFileArgs struct {
	Path string `json:"path"`
}

// ShareFile is the share_file agent tool. A single file is sent as-is; a
// directory is zipped first (the whole directory tree, into one archive —
// see zipDirectory's doc comment) since none of the three delivery
// channels have a concept of sending a folder. Where the result actually
// goes is never this
// function's decision — see FileSender's doc comment.
func ShareFile(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args ShareFileArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.Path) == "" {
		return "", errors.New(T("path boş olamaz", "path cannot be empty"))
	}
	if FileSender == nil {
		return T("Dosya gönderme sistemi hazır değil.", "File sending system not ready."), nil
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf(T("dosya/klasör bulunamadı: %w", "file/folder not found: %w"), err)
	}

	sendPath := fullPath
	displayName := info.Name()
	sendSize := info.Size()
	isTempZip := false

	if info.IsDir() {
		zipPath, err := zipDirectory(fullPath)
		if err != nil {
			return "", fmt.Errorf(T("klasör zip'lenemedi: %w", "could not zip folder: %w"), err)
		}
		sendPath = zipPath
		displayName = info.Name() + ".zip"
		isTempZip = true
		// Cleanup happens below, after DeliverFile — only when it reports
		// the file was consumed (WhatsApp/Telegram already read it in
		// full); the frontend download-link path still needs sendPath to
		// exist on disk for the eventual browser request.
		zipInfo, err := os.Stat(sendPath)
		if err != nil {
			os.Remove(sendPath)
			return "", fmt.Errorf(T("zip boyutu okunamadı: %w", "could not read zip size: %w"), err)
		}
		sendSize = zipInfo.Size()
	}

	if sendSize > maxShareFileBytes {
		if isTempZip {
			os.Remove(sendPath)
		}
		return "", fmt.Errorf(T("dosya çok büyük (%.1f MB, limit %.0f MB)", "file too large (%.1f MB, limit %.0f MB)"), float64(sendSize)/1024/1024, float64(maxShareFileBytes)/1024/1024)
	}

	message, consumed, err := FileSender.DeliverFile(ctx, sendPath, displayName, isTempZip)
	if isTempZip && consumed {
		os.Remove(sendPath)
	}
	if err != nil {
		return "", fmt.Errorf(T("dosya gönderilemedi: %w", "could not send file: %w"), err)
	}
	return message, nil
}

// zipDirectory archives every regular file under dirPath (recursively,
// preserving subdirectory structure relative to dirPath) into a new temp
// file, returned by path. The caller owns cleanup.
//
// Symlinks are skipped entirely, not followed: filepath.Walk itself never
// descends into a symlinked directory, but a symlinked *file* entry would
// still be silently followed by the plain os.Open below, letting a
// directory shared via share_file exfiltrate whatever the backend process
// can read anywhere on disk (e.g. a symlink planted inside an
// agent-writable folder pointing at ~/.ssh/id_rsa) — validatePath's own
// sandbox check (file.go) only ever validates the one top-level path
// share_file was given, not every entry found while recursing through it.
// Same class of escape file.go's own BUG-C1 fix closed for read/write, just
// reached through a different tool.
func zipDirectory(dirPath string) (string, error) {
	tmp, err := os.CreateTemp("", "memo-share-*.zip")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	zw := zip.NewWriter(tmp)
	walkErr := filepath.Walk(dirPath, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if fi.IsDir() {
			return nil
		}
		if fi.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		rel, err := filepath.Rel(dirPath, path)
		if err != nil {
			return err
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			return err
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(w, f)
		return err
	})
	closeErr := zw.Close()
	if walkErr != nil {
		os.Remove(tmp.Name())
		return "", walkErr
	}
	if closeErr != nil {
		os.Remove(tmp.Name())
		return "", closeErr
	}
	return tmp.Name(), nil
}
