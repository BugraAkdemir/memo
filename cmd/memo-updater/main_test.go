// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	ok := func() []string {
		return []string{"/h/.memo", "/h/.memo/.update", "/h/.memo/.update/staging/Memo", "/h/.memo/.update/backup", "/h/.memo/run_memo.sh", "V4.7.0", "http://127.0.0.1:8090/api/version"}
	}
	call := func(a []string) error { return validate(a[0], a[1], a[2], a[3], a[4], a[5], a[6]) }
	if err := call(ok()); err != nil {
		t.Fatalf("good args rejected: %v", err)
	}
	mut := map[string]func([]string){
		"relative install":       func(a []string) { a[0] = ".memo" },
		"work elsewhere":         func(a []string) { a[1] = "/tmp/.update" },
		"work not .update":       func(a []string) { a[1] = "/h/.memo/other" },
		"staged outside work":    func(a []string) { a[2] = "/etc" },
		"staged is work itself":  func(a []string) { a[2] = a[1] },
		"backup outside work":    func(a []string) { a[3] = "/h/.memo/data" },
		"traversal in staged":    func(a []string) { a[2] = "/h/.memo/.update/../data" },
		"launch outside install": func(a []string) { a[4] = "/usr/bin/evil" },
		"empty launch":           func(a []string) { a[4] = "" },
		"bad version":            func(a []string) { a[5] = "latest" },
		"remote health":          func(a []string) { a[6] = "http://example.com/api/version" },
	}
	for name, m := range mut {
		a := ok()
		m(a)
		if err := call(a); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestRun_Usage(t *testing.T) {
	var buf bytes.Buffer
	if code := run(nil, &buf); code != 2 || !strings.Contains(buf.String(), "usage") {
		t.Errorf("no args: code=%d out=%q", code, buf.String())
	}
	buf.Reset()
	if code := run([]string{"apply", "--install", "relative"}, &buf); code != 2 {
		t.Errorf("bad args: code=%d out=%q", code, buf.String())
	}
}
