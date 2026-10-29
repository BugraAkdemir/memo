// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"strings"
)

// SanitizePanic reduces a recovered panic to something safe to count: the TYPE
// name of the panic value (never its message) and up to five of Memo's own
// frames as "<function> <repo-relative file>:<line>".
//
// Everything that could identify a person or a machine is left behind on
// purpose: the panic message, argument values, absolute paths (the directory
// comes from the function's own package path, only the file's base name is
// read from the stack), and every frame that is not Memo's. ok is false when
// the stack holds no Memo frame at all — then there is nothing worth counting.
func SanitizePanic(r any, stack []byte) (CrashAgg, bool) {
	var frames []string
	lines := strings.Split(string(stack), "\n")
	for i := 0; i+1 < len(lines) && len(frames) < 5; i++ {
		fn, ok := frameFunction(lines[i])
		if !ok || !strings.HasPrefix(lines[i+1], "\t") {
			continue
		}
		if f, ok := frameString(fn, lines[i+1]); ok {
			frames = append(frames, f)
		}
	}
	if len(frames) == 0 {
		return CrashAgg{}, false
	}
	sum := sha256.Sum256([]byte(strings.Join(frames, "\n")))
	return CrashAgg{Sig: hex.EncodeToString(sum[:]), Component: "go", ExcType: excTypeName(r), Frames: frames}, true
}

// excTypeName is the panic value's Go type, without a pointer star, or "panic"
// when it is not a plain dotted identifier.
func excTypeName(r any) string {
	t := strings.TrimLeft(fmt.Sprintf("%T", r), "*")
	if excTypeRe.MatchString(t) {
		return t
	}
	return "panic"
}

// frameFunction returns the function of a stack line, without its argument list,
// when it is one of Memo's own and not part of the panic plumbing.
func frameFunction(line string) (string, bool) {
	if line == "" || line[0] == '\t' || line[0] == ' ' || strings.HasPrefix(line, "goroutine ") || strings.HasPrefix(line, "created by ") {
		return "", false
	}
	i := strings.LastIndex(line, "(")
	if i <= 0 || !strings.HasSuffix(line, ")") {
		return "", false
	}
	fn := line[:i]
	switch {
	case strings.HasPrefix(fn, "main."):
		fn = "memo/" + fn
	case !strings.HasPrefix(fn, "memo/"):
		return "", false
	}
	for _, plumbing := range []string{"logx.Recover", "logx.GoRecover", "recoverPanic", "goRecover", "recoverStreamPanic", "internal/telemetry."} {
		if strings.Contains(fn, plumbing) {
			return "", false
		}
	}
	return fn, true
}

// frameString builds "<function> <file>:<line>" from a function and the stack
// line after it ("\t/abs/path/file.go:812 +0x1a"). Only the file's base name is
// taken from the stack; its directory comes from the function's package.
func frameString(fn, fileLine string) (string, bool) {
	s := strings.TrimSpace(fileLine)
	if i := strings.Index(s, " +0x"); i >= 0 {
		s = s[:i]
	}
	colon := strings.LastIndex(s, ":")
	if colon < 0 {
		return "", false
	}
	file, num := s[:colon], s[colon+1:]
	base := path.Base(strings.ReplaceAll(file, `\`, "/"))

	pkg := strings.TrimPrefix(fn, "memo/")
	if slash := strings.LastIndex(pkg, "/"); slash >= 0 {
		pkg = pkg[:slash+1+strings.Index(pkg[slash+1:]+".", ".")]
	} else {
		pkg = "" // memo/main.main: the file is in the repo root
	}
	rel := base
	if pkg != "" {
		rel = pkg + "/" + base
	}
	out := fn + " " + rel + ":" + num
	if !frameRe.MatchString(out) {
		return "", false
	}
	return out, true
}
