package llama

import "testing"

func TestFlashAttnProbeAccepted(t *testing.T) {
	tests := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "unknown flag — old or stripped build",
			output: "error: invalid argument: --flash-attn\nusage: llama-server [options]",
			want:   false,
		},
		{
			name:   "flag now requires a value, aborts before model load",
			output: "error while handling argument \"--flash-attn\": expected value\n",
			want:   false,
		},
		{
			name:   "accepted — reached model load and failed on the probe path",
			output: "llama_model_load: error loading model: failed to open __memo_fa_probe_nonexistent__.gguf\n",
			want:   true,
		},
		{
			name:   "accepted — generic load failure wording",
			output: "common_init_from_params: failed to load model 'x'\nmain: error: unable to load model\n",
			want:   true,
		},
		{
			name:   "accepted — no such file",
			output: "gguf_init_from_file: failed to open '__memo_fa_probe_nonexistent__.gguf': No such file or directory\n",
			want:   true,
		},
		{
			name:   "unrecognised wording — assume unsupported, never risk a bad flag",
			output: "something totally different happened\n",
			want:   false,
		},
		{
			name:   "reject wins even if a load marker also appears",
			output: "loading model...\nerror: invalid argument: --flash-attn\n",
			want:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := flashAttnProbeAccepted(tt.output); got != tt.want {
				t.Errorf("flashAttnProbeAccepted(%q) = %v, want %v", tt.output, got, tt.want)
			}
		})
	}
}
