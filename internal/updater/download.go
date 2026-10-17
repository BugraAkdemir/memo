// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ErrChecksum means the bytes that arrived are not the bytes the signed
// manifest promised. The partial file is deleted; there is nothing to resume.
var ErrChecksum = errors.New("updater: downloaded file does not match its SHA-256")

// ErrNoSpace means the disk cannot hold the download (plus what unpacking and a
// rollback copy need).
var ErrNoSpace = errors.New("updater: not enough free disk space")

// Progress reports a running download.
type Progress struct {
	Done, Total int64
}

// DownloadOptions describes one verified download.
type DownloadOptions struct {
	URL    string
	Dest   string // final path; Dest+".part" is the resumable partial
	SHA256 string // lowercase hex, from the signed manifest
	Size   int64  // from the signed manifest
	HTTP   *http.Client
	// OnProgress is called at most about four times a second.
	OnProgress func(Progress)
}

// Download fetches o.URL into o.Dest and returns only when the file's size and
// SHA-256 match the manifest. An interrupted download resumes from the partial
// file with a Range request; a server that ignores Range simply restarts it.
func Download(ctx context.Context, o DownloadOptions) error {
	if err := checkFile(o.URL, o.SHA256, o.Size); err != nil {
		return err
	}
	client := o.HTTP
	if client == nil {
		client = NewHTTPClient()
	}
	part := o.Dest + ".part"

	have := int64(0)
	if st, err := os.Stat(part); err == nil {
		have = st.Size()
		if have > o.Size {
			_ = os.Remove(part)
			have = 0
		}
	}

	if have < o.Size {
		if err := fetchInto(ctx, client, o, part, have); err != nil {
			return err
		}
	}

	st, err := os.Stat(part)
	if err != nil {
		return err
	}
	if st.Size() != o.Size {
		_ = os.Remove(part)
		return fmt.Errorf("updater: got %d bytes, the manifest says %d", st.Size(), o.Size)
	}
	sum, err := fileSHA256(part)
	if err != nil {
		return err
	}
	if sum != strings.ToLower(o.SHA256) {
		_ = os.Remove(part)
		return ErrChecksum
	}
	return os.Rename(part, o.Dest)
}

func fetchInto(ctx context.Context, client *http.Client, o DownloadOptions, part string, have int64) error {
	// The partial file counts toward what we already hold; only the rest needs
	// room (and the caller has checked the unpack/rollback headroom separately).
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.URL, nil)
	if err != nil {
		return err
	}
	if have > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", have))
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch resp.StatusCode {
	case http.StatusOK:
		have = 0 // the server ignored Range: start over
		flags |= os.O_TRUNC
	case http.StatusPartialContent:
		if !strings.HasPrefix(resp.Header.Get("Content-Range"), fmt.Sprintf("bytes %d-", have)) {
			return errors.New("updater: server answered a different byte range than asked")
		}
		flags |= os.O_APPEND
	case http.StatusRequestedRangeNotSatisfiable:
		_ = os.Remove(part)
		return errors.New("updater: partial download was stale; retry")
	default:
		return fmt.Errorf("updater: GET %s: HTTP %d", redact(o.URL), resp.StatusCode)
	}
	if resp.ContentLength >= 0 && have+resp.ContentLength != o.Size {
		return fmt.Errorf("updater: server will send %d bytes, the manifest says %d", have+resp.ContentLength, o.Size)
	}

	f, err := os.OpenFile(part, flags, 0o600)
	if err != nil {
		return err
	}
	// +1 so a body longer than promised is noticed rather than silently cut.
	n, copyErr := io.Copy(f, &progressReader{
		r:     io.LimitReader(resp.Body, o.Size-have+1),
		done:  have,
		total: o.Size,
		fn:    o.OnProgress,
	})
	if cerr := f.Close(); copyErr == nil {
		copyErr = cerr
	}
	if copyErr != nil {
		return copyErr
	}
	if have+n != o.Size {
		return fmt.Errorf("updater: received %d bytes, the manifest says %d", have+n, o.Size)
	}
	return nil
}

type progressReader struct {
	r           io.Reader
	done, total int64
	fn          func(Progress)
	last        time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.done += int64(n)
	if p.fn != nil && (time.Since(p.last) > 250*time.Millisecond || err == io.EOF) {
		p.last = time.Now()
		p.fn(Progress{Done: p.done, Total: p.total})
	}
	return n, err
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SHA256File is the exported form, for tools that publish manifests.
func SHA256File(path string) (string, error) { return fileSHA256(path) }

// EnsureSpace fails with ErrNoSpace unless dir's filesystem has at least need
// bytes free.
func EnsureSpace(dir string, need int64) error {
	free, err := freeBytes(dir)
	if err != nil {
		// Not being able to ask is not a reason to refuse; the download itself
		// will fail loudly if the disk really is full.
		return nil
	}
	if need > 0 && free < uint64(need) {
		return fmt.Errorf("%w: need about %d MB, %d MB free", ErrNoSpace, need>>20, free>>20)
	}
	return nil
}
