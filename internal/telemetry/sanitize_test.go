// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

// A realistic Go panic stack, as runtime/debug.Stack prints it, from a machine
// whose user is "alice" and whose panic message carries a secret.
const sampleStack = `goroutine 41 [running]:
runtime/debug.Stack()
	/usr/local/go/src/runtime/debug/stack.go:26 +0x5e
memo/internal/logx.Recover({0x9a1b2c, 0xf})
	/home/alice/Documents/memo/internal/logx/logx.go:74 +0x3a
panic({0x8a1b00, 0xc0001a2b40})
	/usr/local/go/src/runtime/panic.go:785 +0x132
memo/internal/app.(*App).finishStream(0xc0003a4000, {0x9b2c3d, 0x5}, 0x1, 0x0, {0xc000123456, 0x1c})
	/home/alice/Documents/memo/internal/app/llm.go:2042 +0x1a9
memo/internal/app.(*App).sendMessageStreamCore.func2()
	/home/alice/Documents/memo/internal/app/chat.go:455 +0x77
memo/internal/webserver.(*Server).handleSendStream(0xc000010000, {0xa1b2c3, 0xc00002a000}, 0xc000150000)
	C:/Users/alice/memo/internal/webserver/handlers_flutter.go:812 +0x9c
net/http.HandlerFunc.ServeHTTP(0x1, {0x2, 0x3}, 0x4)
	/usr/local/go/src/net/http/server.go:2220 +0x29
main.main()
	/home/alice/Documents/memo/main.go:120 +0x44
`

func TestSanitizePanic_KeepsTheTypeAndMemoFramesOnly(t *testing.T) {
	got, ok := SanitizePanic(errors.New("SECRET-CANARY: could not open /home/alice/secret-project/notes.txt"), []byte(sampleStack))
	if !ok {
		t.Fatal("a panic in Memo code must be reportable")
	}
	if got.Component != "go" || got.ExcType != "errors.errorString" {
		t.Errorf("type = %+v (only the TYPE name may be kept, never the message)", got)
	}
	want := []string{
		"memo/internal/app.(*App).finishStream internal/app/llm.go:2042",
		"memo/internal/app.(*App).sendMessageStreamCore.func2 internal/app/chat.go:455",
		"memo/internal/webserver.(*Server).handleSendStream internal/webserver/handlers_flutter.go:812",
		"memo/main.main main.go:120",
	}
	if strings.Join(got.Frames, "\n") != strings.Join(want, "\n") {
		t.Fatalf("frames =\n  %s\nwant\n  %s", strings.Join(got.Frames, "\n  "), strings.Join(want, "\n  "))
	}
	sum := sha256.Sum256([]byte(strings.Join(got.Frames, "\n")))
	if got.Sig != hex.EncodeToString(sum[:]) {
		t.Error("the signature must be the hash of the frames")
	}
}

func TestSanitizePanic_NothingPrivateSurvives(t *testing.T) {
	got, _ := SanitizePanic("SECRET-CANARY in /home/alice/secret-project", []byte(sampleStack))
	out := got.ExcType + got.Component + got.Sig + strings.Join(got.Frames, "|")
	for _, leak := range []string{"SECRET-CANARY", "alice", "/home", "C:/", "Users", "secret-project", "0xc000", "+0x"} {
		if strings.Contains(out, leak) {
			t.Errorf("%q leaked into %q", leak, out)
		}
	}
	if got.ExcType != "string" {
		t.Errorf("a panic(\"text\") is reported as its type, string, got %q", got.ExcType)
	}
}

func TestSanitizePanic_AtMostFiveFramesAndAlwaysValidForTheServer(t *testing.T) {
	var b strings.Builder
	b.WriteString("goroutine 1 [running]:\n")
	for i := 0; i < 12; i++ {
		b.WriteString("memo/internal/app.(*App).step" + string(rune('A'+i)) + "(0x1)\n\t/home/bob/memo/internal/app/x.go:" + string(rune('1'+i%9)) + " +0x1\n")
	}
	got, ok := SanitizePanic(errors.New("boom"), []byte(b.String()))
	if !ok || len(got.Frames) != 5 {
		t.Fatalf("want 5 frames, got %d (%v)", len(got.Frames), ok)
	}
	if _, valid := crashRow(&got); !valid {
		t.Fatalf("the sanitized crash must pass the collector's own checks: %+v", got)
	}
}

func TestSanitizePanic_APanicOutsideMemoCodeIsNotReported(t *testing.T) {
	stack := "goroutine 9 [running]:\nruntime/debug.Stack()\n\t/usr/local/go/src/runtime/debug/stack.go:26 +0x5e\ngithub.com/some/dep.Do(0x1)\n\t/home/alice/go/pkg/mod/github.com/some/dep/x.go:10 +0x1\n"
	if _, ok := SanitizePanic(errors.New("x"), []byte(stack)); ok {
		t.Fatal("a stack with no Memo frame has nothing to report and must not be sent")
	}
}

func TestSanitizePanic_WeirdTypeNamesFallBackToPanic(t *testing.T) {
	type generic[T any] struct{}
	got, ok := SanitizePanic(generic[int]{}, []byte(sampleStack))
	if !ok || !excTypeRe.MatchString(got.ExcType) {
		t.Fatalf("type %q is not something the server accepts", got.ExcType)
	}
}

func TestSanitizePanic_AWindowsPathNeverLeaksEither(t *testing.T) {
	stack := "goroutine 1 [running]:\nmemo/internal/app.(*App).run(0x1)\n\tC:/Users/alice/Documents/memo/internal/app/app.go:77 +0x1\n"
	got, ok := SanitizePanic(errors.New("x"), []byte(stack))
	if !ok || len(got.Frames) != 1 || got.Frames[0] != "memo/internal/app.(*App).run internal/app/app.go:77" {
		t.Fatalf("frames = %v", got.Frames)
	}
}
