// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"memo/internal/project"
)

// projectAPI is the optional bridge behind /api/projects/*. An optional
// interface, like chatProjectSetter, so every existing bridge and test stub
// keeps compiling.
type projectAPI interface {
	ProjectStatus(path string) project.UIStatus
	SetupProject(path string) (*project.ScaffoldResult, error)
	DeclineProject(path string, declined bool) error
}

type projectPathRequest struct {
	Path     string `json:"path"`
	Declined *bool  `json:"declined"`
}

func (s *Server) projectRequest(w http.ResponseWriter, r *http.Request) (projectAPI, projectPathRequest, bool) {
	var req projectPathRequest
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return nil, req, false
	}
	api, ok := s.fullBridge.(projectAPI)
	if !ok {
		http.Error(w, "not supported", http.StatusNotImplemented)
		return nil, req, false
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Path == "" {
		http.Error(w, "path required", http.StatusBadRequest)
		return nil, req, false
	}
	return api, req, true
}

// handleProjectStatus: POST {path} -> whether the folder can be a Memo project,
// is set up, and whether the UI should offer setting it up. Writes nothing.
func (s *Server) handleProjectStatus(w http.ResponseWriter, r *http.Request) {
	api, req, ok := s.projectRequest(w, r)
	if !ok {
		return
	}
	writeJSON(w, api.ProjectStatus(req.Path))
}

// handleProjectSetup: POST {path} -> creates `.memo/` and the starter files
// (never overwriting anything). 403 when the folder can never be a project
// (home, Memo's own folders, a foreign .memo) or the feature is off.
func (s *Server) handleProjectSetup(w http.ResponseWriter, r *http.Request) {
	api, req, ok := s.projectRequest(w, r)
	if !ok {
		return
	}
	res, err := api.SetupProject(req.Path)
	if err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, project.ErrForbiddenRoot) || errors.Is(err, project.ErrForeignDir) {
			code = http.StatusForbidden
		} else if st := api.ProjectStatus(req.Path); !st.Enabled {
			code = http.StatusForbidden
		}
		http.Error(w, err.Error(), code)
		return
	}
	writeJSON(w, res)
}

// handleProjectDecline: POST {path, declined?} -> remembers "never set up Memo
// in this folder" (declined defaults to true; false undoes it).
func (s *Server) handleProjectDecline(w http.ResponseWriter, r *http.Request) {
	api, req, ok := s.projectRequest(w, r)
	if !ok {
		return
	}
	declined := true
	if req.Declined != nil {
		declined = *req.Declined
	}
	if err := api.DeclineProject(req.Path, declined); err != nil {
		code := http.StatusInternalServerError
		if errors.Is(err, project.ErrForbiddenRoot) {
			code = http.StatusForbidden
		}
		http.Error(w, err.Error(), code)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}
