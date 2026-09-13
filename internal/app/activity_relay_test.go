package app

import (
	"testing"

	"memo/internal/api"
	"memo/internal/models"
)

func TestActivityRelay_ContentReportsGenerating(t *testing.T) {
	resetActivity(t)
	in := make(chan api.StreamChunk, 4)
	out := activityRelay(in)

	in <- api.StreamChunk{Content: "merhaba"}
	<-out

	got := (&App{}).GetActivityStatus()
	if got.State != models.ActivityGenerating {
		t.Errorf("State = %q, want generating", got.State)
	}
	close(in)
	<-out // drain to close
}

func TestActivityRelay_DoneChunkReportsActivityDone(t *testing.T) {
	resetActivity(t)
	in := make(chan api.StreamChunk, 4)
	out := activityRelay(in)

	in <- api.StreamChunk{Content: "merhaba"}
	<-out
	in <- api.StreamChunk{Done: true}
	<-out
	close(in)

	got := (&App{}).GetActivityStatus()
	if got.State != models.ActivityDone {
		t.Errorf("State = %q, want done", got.State)
	}
}

func TestActivityRelay_PassesChunksThroughUnchanged(t *testing.T) {
	resetActivity(t)
	in := make(chan api.StreamChunk, 4)
	out := activityRelay(in)

	in <- api.StreamChunk{Content: "hello"}
	in <- api.StreamChunk{Content: " world"}
	in <- api.StreamChunk{Done: true}
	close(in)

	var got []api.StreamChunk
	for chunk := range out {
		got = append(got, chunk)
	}
	if len(got) != 3 || got[0].Content != "hello" || got[1].Content != " world" || !got[2].Done {
		t.Fatalf("unexpected relayed chunks: %+v", got)
	}
}

func TestActivityRelay_DoesNotReArmWithinReArmWindow(t *testing.T) {
	resetActivity(t)
	in := make(chan api.StreamChunk, 4)
	out := activityRelay(in)

	in <- api.StreamChunk{Content: "a"}
	<-out
	firstSince := (&App{}).GetActivityStatus().Since

	in <- api.StreamChunk{Content: "b"}
	<-out
	secondSince := (&App{}).GetActivityStatus().Since

	if !secondSince.Equal(firstSince) {
		t.Errorf("activity was re-armed within the 3s window: first=%v second=%v", firstSince, secondSince)
	}
	close(in)
	<-out
}
