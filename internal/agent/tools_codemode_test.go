package agent

import "testing"

func TestForCodeMode_DropsNonCodeToolsAndKeepsTheCodingOnes(t *testing.T) {
	r := NewRegistry()
	all := map[string]bool{}
	for _, d := range r.ToOpenAITools(nil) {
		all[d.Function.Name] = true
	}
	code := map[string]bool{}
	for _, d := range ForCodeMode(r.ToOpenAITools(nil)) {
		code[d.Function.Name] = true
	}

	for _, n := range []string{"whatsapp_send", "whatsapp_search", "whatsapp_latest", "whatsapp_messages",
		"get_calendar_events", "create_routine", "list_routines", "cancel_routine", "configure_provider", "self_clone"} {
		// (WhatsApp tools are registered by the app at startup, not by NewRegistry.)
		if !all[n] && n != "whatsapp_send" && n != "whatsapp_search" && n != "whatsapp_latest" && n != "whatsapp_messages" {
			t.Errorf("%s must still exist in the registry (ordinary agent chats use it)", n)
		}
		if code[n] {
			t.Errorf("%s must not be offered in Code Mode", n)
		}
	}
	for _, n := range []string{"read_file", "write_file", "edit_file", "insert_line", "delete_lines", "search_files",
		"search_content", "list_directory", "run_command", "get_file_info", "web_search", "fetch_page",
		"start_self_driving_task", "create_task_md", "edit_task_md", "save_code_plan", "get_task_status"} {
		if !code[n] {
			t.Errorf("%s must stay available in Code Mode", n)
		}
	}
}

func TestSearchContentIsAvailableToReadOnlySubAgents(t *testing.T) {
	r := NewReadOnlyRegistry()
	if _, ok := r.Get("search_content"); !ok {
		t.Fatal("read-only sub-agents need search_content to look code up")
	}
}

func TestToolDescriptionsSteerTowardGoodHabits(t *testing.T) {
	r := NewRegistry()
	for name, must := range map[string]string{
		"search_files": "search_content",
		"write_file":   "edit_file",
		"edit_file":    "EXACTLY",
		"read_file":    "before editing",
	} {
		tool, ok := r.Get(name)
		if !ok {
			t.Fatalf("no %s", name)
		}
		if !contains(tool.Description, must) {
			t.Errorf("%s description %q should mention %q", name, tool.Description, must)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
