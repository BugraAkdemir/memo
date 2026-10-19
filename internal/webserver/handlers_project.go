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
	GetProjectPermissions(path string) (*project.Permissions, project.PermissionsStatus, error)
	SaveProjectPermissions(path string, perms *project.Permissions) error
	TrustProjectPermissions(path string) (string, error)
}

type projectPathRequest struct {
	Path        string               `json:"path"`
	Declined    *bool                `json:"declined"`
	Permissions *project.Permissions `json:"permissions,omitempty"`
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
// (home, Memo's own folders, a foreign .memo).
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

// handleProjectPermissions: POST {path} -> returns permissions and trust status.
func (s *Server) handleProjectPermissions(w http.ResponseWriter, r *http.Request) {
	api, req, ok := s.projectRequest(w, r)
	if !ok {
		return
	}
	perms, status, err := api.GetProjectPermissions(req.Path)
	if err != nil {
		http.Error(w, err.Error(), permissionsErrorStatus(err, http.StatusInternalServerError))
		return
	}
	writeJSON(w, map[string]any{
		"permissions": perms,
		"status":      status,
	})
}

// handleProjectPermissionsSave: POST {path, permissions} -> saves permissions and trusts them.
func (s *Server) handleProjectPermissionsSave(w http.ResponseWriter, r *http.Request) {
	api, req, ok := s.projectRequest(w, r)
	if !ok {
		return
	}
	if req.Permissions == nil {
		http.Error(w, "permissions required", http.StatusBadRequest)
		return
	}
	if err := api.SaveProjectPermissions(req.Path, req.Permissions); err != nil {
		http.Error(w, err.Error(), permissionsErrorStatus(err, http.StatusBadRequest))
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// handleProjectPermissionsTrust: POST {path} -> trusts existing permissions.json hash.
func (s *Server) handleProjectPermissionsTrust(w http.ResponseWriter, r *http.Request) {
	api, req, ok := s.projectRequest(w, r)
	if !ok {
		return
	}
	hash, err := api.TrustProjectPermissions(req.Path)
	if err != nil {
		http.Error(w, err.Error(), permissionsErrorStatus(err, http.StatusBadRequest))
		return
	}
	writeJSON(w, map[string]string{"ok": "true", "hash": hash})
}

// permissionsErrorStatus maps a permissions error to a status: a folder that can
// never be a project (or a foreign .memo) is 403; a rule that breaks the safety
// constraints is 400; anything else is def.
func permissionsErrorStatus(err error, def int) int {
	switch {
	case errors.Is(err, project.ErrForbiddenRoot), errors.Is(err, project.ErrForeignDir):
		return http.StatusForbidden
	case errors.Is(err, project.ErrInvalidRule):
		return http.StatusBadRequest
	}
	return def
}
