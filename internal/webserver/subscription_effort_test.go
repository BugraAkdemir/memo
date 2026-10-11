package webserver

import (
	"context"
	"encoding/json"
	"memo/internal/models"
	"net/http"
	"net/http/httptest"
	"testing"
)

type subscriptionEffortBridge struct{ *swarmStubBridge }

func (b *subscriptionEffortBridge) ListProviderModels(ctx context.Context, name string) ([]models.ProviderModel, string, error) {
	if name != "Subscriptions" {
		panic("wrong provider")
	}
	return []models.ProviderModel{{ID: "listed-model", ContextLength: 1000000, EffortLevels: []string{"medium", "high"}}}, "listed-model", nil
}
func TestSubscriptionEffortsOnlyForAnAccountListedModel(t *testing.T) {
	s := New(&subscriptionEffortBridge{&swarmStubBridge{}})
	for _, model := range []string{"listed-model", "catalog-only-model"} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=subscriptions&model="+model, nil)
		s.handleProviderEffortLevels(w, r)
		if w.Code != http.StatusOK {
			t.Fatalf("status=%d", w.Code)
		}
		var got struct {
			Levels []string `json:"levels"`
		}
		if e := json.Unmarshal(w.Body.Bytes(), &got); e != nil {
			t.Fatal(e)
		}
		want := 0
		if model == "listed-model" {
			want = 2
		}
		if len(got.Levels) != want {
			t.Fatalf("%s levels=%v", model, got.Levels)
		}
	}
}
