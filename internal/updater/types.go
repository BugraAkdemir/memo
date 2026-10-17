// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

// Update modes: how this particular install can be updated.
const (
	// ModeAuto: Memo downloads the release and swaps it in itself (Linux and
	// macOS installs made by scripts/get-memo.sh, which have a launcher script
	// the updater can restart Memo with).
	ModeAuto = "auto"
	// ModeInstaller: Windows. Memo downloads Setup.exe and starts it; the user
	// finishes in the installer.
	ModeInstaller = "installer"
	// ModeManual: AppImage, deb, Docker, a source checkout, a platform with no
	// published build. The person is told a version exists and where to get it.
	ModeManual = "manual"
)

// Status states.
const (
	StateIdle        = "idle"
	StateDownloading = "downloading"
	StateVerifying   = "verifying"
	StateInstalling  = "installing"
	StateRestarting  = "restarting"
	StateDone        = "done"
	StateError       = "error"
)

// ComponentInfo is one engine component as the update check sees it.
type ComponentInfo struct {
	Name      string `json:"name"`      // llama | whisper | cliproxy
	Folder    string `json:"folder"`    // the engine folder in use, "cpu", "amd", …
	Installed string `json:"installed"` // "" when unknown
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	// Reason says why Available is false, as a short code the UI can translate:
	// up_to_date | unknown_installed | not_for_this_machine | needs_newer_app |
	// not_ours | check_failed.
	Reason      string `json:"reason,omitempty"`
	RequiresApp string `json:"requires_app,omitempty"`
}

// Info is the answer to "is there an update?".
type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	Required  bool   `json:"required"`
	NoteTR    string `json:"note_tr,omitempty"`
	NoteEN    string `json:"note_en,omitempty"`
	Mode      string `json:"mode"`
	Platform  string `json:"platform"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	// DownloadURL is where a person goes when Mode is manual.
	DownloadURL string          `json:"download_url,omitempty"`
	Components  []ComponentInfo `json:"components"`
	Error       string          `json:"error,omitempty"`
	CheckedAt   int64           `json:"checked_at,omitempty"`
}

// Status is the progress of a running update.
type Status struct {
	State     string `json:"state"`
	Kind      string `json:"kind,omitempty"` // app | component
	Component string `json:"component,omitempty"`
	Done      int64  `json:"done"`
	Total     int64  `json:"total"`
	Error     string `json:"error,omitempty"`
}
