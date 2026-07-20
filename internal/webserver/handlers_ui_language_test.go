// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleUILanguage_NilFullBridge(t *testing.T) {
	s := newMockServer(&mockBridge{})
	// fullBridge intentionally nil

	req := httptest.NewRequest(http.MethodGet, "/api/system-prompt/ui-language", nil)
	w := httptest.NewRecorder()
	s.handleUILanguage(w, req)
	if w.Code != http.StatusNotImplemented {
		t.Errorf("status = %d, want %d (body %q)", w.Code, http.StatusNotImplemented, w.Body.String())
	}
}

func TestHandleUILanguage_GetReturnsCurrentValue(t *testing.T) {
	stub := &swarmStubBridge{uiLanguage: "en"}
	s := New(stub)
	s.fullBridge = stub

	req := httptest.NewRequest(http.MethodGet, "/api/system-prompt/ui-language", nil)
	w := httptest.NewRecorder()
	s.handleUILanguage(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"en"`) {
		t.Errorf("body = %q, want it to contain the language %q", w.Body.String(), "en")
	}
}

func TestHandleUILanguage_PutPersistsAndGetReflectsIt(t *testing.T) {
	stub := &swarmStubBridge{}
	s := New(stub)
	s.fullBridge = stub

	putReq := httptest.NewRequest(http.MethodPut, "/api/system-prompt/ui-language", strings.NewReader(`{"language":"en"}`))
	putW := httptest.NewRecorder()
	s.handleUILanguage(putW, putReq)
	if putW.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, want 200 (body %q)", putW.Code, putW.Body.String())
	}
	if stub.uiLanguage != "en" {
		t.Fatalf("stub.uiLanguage = %q after PUT, want %q", stub.uiLanguage, "en")
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/system-prompt/ui-language", nil)
	getW := httptest.NewRecorder()
	s.handleUILanguage(getW, getReq)
	if !strings.Contains(getW.Body.String(), `"en"`) {
		t.Errorf("GET after PUT body = %q, want it to reflect the just-set language", getW.Body.String())
	}
}

func TestHandleUILanguage_PutRejectsInvalidLanguage(t *testing.T) {
	stub := &swarmStubBridge{}
	s := New(stub)
	s.fullBridge = stub

	req := httptest.NewRequest(http.MethodPut, "/api/system-prompt/ui-language", strings.NewReader(`{"language":"fr"}`))
	w := httptest.NewRecorder()
	s.handleUILanguage(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 for an unsupported language (body %q)", w.Code, w.Body.String())
	}
	if stub.uiLanguage != "" {
		t.Errorf("stub.uiLanguage = %q, want untouched after a rejected PUT", stub.uiLanguage)
	}
}
