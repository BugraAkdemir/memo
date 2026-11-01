package twin

import (
	"context"
	"testing"
)

func TestReplyModelPersistsIndependentlyOfAutonomyPreferences(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := s.ReplyModel(ctx); err != nil || got.Provider != "" || got.Model != "" {
		t.Fatal(got, err)
	}
	choice := ReplyModel{Provider: "writer", Model: "reply-model"}
	if err = s.SaveReplyModel(ctx, choice); err != nil {
		t.Fatal(err)
	}
	p, err := s.Preferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.Autonomy.Mail = true
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if got, err := s.ReplyModel(ctx); err != nil || got != choice {
		t.Fatal(got, err)
	}
	if err = s.SaveReplyModel(ctx, ReplyModel{Provider: "only-half"}); err == nil {
		t.Fatal("incomplete pair accepted")
	}
	if err = s.SaveReplyModel(ctx, ReplyModel{}); err != nil {
		t.Fatal(err)
	}
}
