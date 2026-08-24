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
// consumed reports whether fullPath has been fully read/handed off and may
// be deleted by the caller if it was a temp file share_file created (a
// zip); false means the implementation is still holding onto fullPath for
// later (the frontend download link case) and the caller must leave it
// alone.
var FileSender interface {
	DeliverFile(ctx context.Context, fullPath, displayName string) (message string, consumed bool, err error)
}

// ShareFileArgs is share_file's only argument — see the FileSender doc
// comment above for why there is no destination/channel parameter.
type ShareFileArgs struct {
	Path string `json:"path"`
}

// ShareFile is the share_file agent tool. A single file is sent as-is; a
// directory is zipped first (the whole directory, flattened into one
// archive) since none of the three delivery channels have a concept of
// sending a folder. Where the result actually goes is never this
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
	}

	if zipInfo, err := os.Stat(sendPath); err == nil && zipInfo.Size() > maxShareFileBytes {
		if isTempZip {
			os.Remove(sendPath)
		}
		return "", fmt.Errorf(T("dosya çok büyük (%.1f MB, limit %.0f MB)", "file too large (%.1f MB, limit %.0f MB)"), float64(zipInfo.Size())/1024/1024, float64(maxShareFileBytes)/1024/1024)
	}

	message, consumed, err := FileSender.DeliverFile(ctx, sendPath, displayName)
	if isTempZip && consumed {
		os.Remove(sendPath)
	}
	if err != nil {
		return "", fmt.Errorf(T("dosya gönderilemedi: %w", "could not send file: %w"), err)
	}
	return message, nil
}

// zipDirectory archives every file under dirPath (recursively, flattened
// into a single zip whose internal paths are relative to dirPath) into a
// new temp file, returned by path. The caller owns cleanup.
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
