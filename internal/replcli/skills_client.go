package replcli

import (
	"context"
	"net/http"
	"net/url"
)

// SkillInfo is one installed skill.
type SkillInfo struct {
	Name        string
	Description string
}

// ListSkills returns the installed skills.
func (c *Client) ListSkills(ctx context.Context) ([]SkillInfo, error) {
	// The backend marshals skill.SkillDefinition as is: the manifest sits under
	// "Manifest" with lower-case name/description inside.
	var defs []struct {
		Manifest struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/skills/list", nil, &defs); err != nil {
		return nil, err
	}
	out := make([]SkillInfo, 0, len(defs))
	for _, d := range defs {
		if d.Manifest.Name != "" {
			out = append(out, SkillInfo{Name: d.Manifest.Name, Description: d.Manifest.Description})
		}
	}
	return out, nil
}

// ChatSkills returns the names of the skills active in ONE chat. Skills are
// scoped to a chat, so a REPL session's skills are its own: another chat, in the
// desktop app or another terminal, is unaffected.
func (c *Client) ChatSkills(ctx context.Context, chatID string) ([]string, error) {
	var resp struct {
		Names []string `json:"names"`
	}
	if err := c.doJSON(ctx, http.MethodGet, "/api/skills/active-list?chat_id="+url.QueryEscape(chatID), nil, &resp); err != nil {
		return nil, err
	}
	return resp.Names, nil
}

// SetChatSkills replaces the set of skills active in one chat.
func (c *Client) SetChatSkills(ctx context.Context, chatID string, names []string) error {
	if names == nil {
		names = []string{}
	}
	return c.doJSON(ctx, http.MethodPut, "/api/skills/active", map[string]any{"chat_id": chatID, "names": names}, nil)
}
