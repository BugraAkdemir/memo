package livemode

import "testing"

func TestEngineTypeIsValid(t *testing.T) {
	valid := []EngineType{EngineLocal, EngineGoogleLive, EngineOpenAIRealtime, EngineElevenLabs, EngineCustom}
	for _, e := range valid {
		if !e.IsValid() {
			t.Errorf("expected %s to be valid", e)
		}
	}
	if EngineType("bogus").IsValid() {
		t.Error("expected an unknown engine type to be invalid")
	}
}

func TestEngineConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     EngineConfig
		wantErr bool
	}{
		{"local rejected", EngineConfig{Type: EngineLocal}, true},
		{"unknown type", EngineConfig{Type: "bogus"}, true},
		{"google_live missing api key", EngineConfig{Type: EngineGoogleLive}, true},
		{"google_live valid", EngineConfig{Type: EngineGoogleLive, APIKey: "k"}, false},
		{"openai_realtime valid", EngineConfig{Type: EngineOpenAIRealtime, APIKey: "k"}, false},
		{"elevenlabs missing api key", EngineConfig{Type: EngineElevenLabs}, true},
		{"custom missing base_url", EngineConfig{Type: EngineCustom, APIKey: "k"}, true},
		{"custom valid without api key", EngineConfig{Type: EngineCustom, BaseURL: "http://localhost:9999"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}
