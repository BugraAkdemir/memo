// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// schemaPaths lists every field a Report can carry as a dotted path
// (usage[].provider, purpose.{kind}.turns, milestones.installed, ...), read off
// the struct tags. docs/TELEMETRY.md must list exactly these, so the page the
// person reads can never lag behind (or run ahead of) what is actually sent.
func schemaPaths(t reflect.Type, prefix string, out map[string]bool) {
	switch t.Kind() {
	case reflect.Pointer:
		schemaPaths(t.Elem(), prefix, out)
	case reflect.Slice:
		schemaPaths(t.Elem(), prefix+"[]", out)
	case reflect.Map:
		schemaPaths(t.Elem(), prefix+".{key}", out)
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			name := strings.Split(t.Field(i).Tag.Get("json"), ",")[0]
			if name == "" || name == "-" {
				continue
			}
			p := name
			if prefix != "" {
				p = prefix + "." + name
			}
			schemaPaths(t.Field(i).Type, p, out)
		}
	default:
		out[prefix] = true
	}
}

var docFieldRe = regexp.MustCompile("(?m)^- `([^`]+)`")

func documentedPaths(t *testing.T, rel string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", rel))
	if err != nil {
		t.Fatalf("%s: %v", rel, err)
	}
	text := string(b)
	begin, end := strings.Index(text, "<!-- fields:begin -->"), strings.Index(text, "<!-- fields:end -->")
	if begin < 0 || end < begin {
		t.Fatalf("%s has no <!-- fields:begin --> ... <!-- fields:end --> block", rel)
	}
	got := map[string]bool{}
	for _, m := range docFieldRe.FindAllStringSubmatch(text[begin:end], -1) {
		if got[m[1]] {
			t.Errorf("%s lists %s twice", rel, m[1])
		}
		got[m[1]] = true
	}
	return got
}

func TestDocs_ListExactlyTheFieldsTheReportCarries(t *testing.T) {
	want := map[string]bool{}
	schemaPaths(reflect.TypeOf(Report{}), "", want)
	if len(want) < 40 {
		t.Fatalf("only %d schema paths found - the reflection walk is broken", len(want))
	}
	for _, rel := range []string{"docs/TELEMETRY.md"} {
		got := documentedPaths(t, rel)
		var missing, stale []string
		for p := range want {
			if !got[p] {
				missing = append(missing, p)
			}
		}
		for p := range got {
			if !want[p] {
				stale = append(stale, p)
			}
		}
		sort.Strings(missing)
		sort.Strings(stale)
		if len(missing) > 0 {
			t.Errorf("%s does not mention fields that are sent: %v", rel, missing)
		}
		if len(stale) > 0 {
			t.Errorf("%s lists fields that are not sent any more: %v", rel, stale)
		}
	}
}
