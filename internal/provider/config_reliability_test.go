package provider

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProviderSettingsSurviveReload(t *testing.T) {
	for _, name := range []string{"API", "Subscriptions"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "providers.json")
			key := make([]byte, 32)
			cm := NewConfigManager(path, key)
			want := ProviderConfig{Type: ProviderCustom, Name: name, Model: "chat", Models: []ConfiguredModel{{ID: "chat"}, {ID: "other"}}, BaseURL: "http://localhost:1/v1", APIKey: "private-test-key", Enabled: true, EffortLevel: "high", ContextTokens: 1000000}
			if err := cm.Set(want); err != nil {
				t.Fatal(err)
			}
			got := NewConfigManager(path, key).GetAll()
			if len(got) != 1 || !reflect.DeepEqual(got[0], want) {
				t.Fatalf("settings did not round-trip: %+v", got)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(data), want.APIKey) {
				t.Fatal("plaintext API key persisted")
			}
		})
	}
}

func TestLegacyProviderSettingsLoadDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	if err := os.WriteFile(path, []byte(`{"providers":[{"type":"custom","name":"legacy","model":"chat","enabled":true}]}`), 0600); err != nil {
		t.Fatal(err)
	}
	got := NewConfigManager(path, make([]byte, 32)).GetAll()
	if len(got) != 1 || got[0].EffortLevel != "" || got[0].ContextTokens != 0 {
		t.Fatalf("legacy defaults changed: %+v", got)
	}
}

func TestProviderWriteFailureRollsBack(t *testing.T) {
	for _, operation := range []string{"add", "update", "delete-name", "delete-type", "save"} {
		t.Run(operation, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "providers.json")
			cm := NewConfigManager(path, make([]byte, 32))
			initial := ProviderConfig{Type: ProviderCustom, Name: "first", Model: "chat", Models: []ConfiguredModel{{ID: "chat"}}}
			if err := cm.Set(initial); err != nil {
				t.Fatal(err)
			}
			if err := cm.Set(ProviderConfig{Type: ProviderOpenAI, Name: "second", Model: "other"}); err != nil {
				t.Fatal(err)
			}
			before := cm.GetAll()
			// A directory cannot be replaced by AtomicWrite, on any platform.
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			var err error
			switch operation {
			case "add":
				err = cm.Set(ProviderConfig{Type: ProviderCustom, Name: "new", Model: "new"})
			case "update":
				initial.Model = "changed"
				err = cm.Set(initial)
			case "delete-name":
				err = cm.Delete(ProviderCustom, "first")
			case "delete-type":
				err = cm.Delete(ProviderCustom)
			case "save":
				err = cm.Save()
			}
			if err == nil {
				t.Fatal("failed write reported success")
			}
			if !reflect.DeepEqual(cm.GetAll(), before) {
				t.Fatal("failed write changed active settings")
			}
		})
	}
}

func TestProviderEncryptionFailureDoesNotDropConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")
	cm := NewConfigManager(path, make([]byte, 32))
	cfg := ProviderConfig{Type: ProviderCustom, Name: "first", Model: "chat", APIKey: "secret"}
	if err := cm.Set(cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cm.masterKey = []byte("invalid")
	cfg.Model = "changed"
	if err := cm.Set(cfg); err == nil {
		t.Fatal("encryption failure reported success")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) || cm.GetAll()[0].Model != "chat" {
		t.Fatal("encryption failure lost provider")
	}
}

func TestConnectionRequiresSelectedModelResponse(t *testing.T) {
	for _, status := range []int{200, 404, 401, 429, 503} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/v1/models" {
					json.NewEncoder(w).Encode(map[string]any{"data": []any{map[string]string{"id": "other"}}})
					return
				}
				if status == 200 {
					w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
					return
				}
				w.WriteHeader(status)
				w.Write([]byte(`{"error":{"message":"selected model failed"}}`))
			}))
			defer srv.Close()
			cm := NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
			err := cm.TestConnection(&ProviderConfig{Type: ProviderCustom, BaseURL: srv.URL + "/v1", Model: "selected"})
			if status == 200 && err != nil {
				t.Fatal(err)
			}
			if status != 200 && err == nil {
				t.Fatal("successful catalog hid failed chat")
			}
		})
	}
}
