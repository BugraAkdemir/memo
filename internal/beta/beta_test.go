// SPDX-License-Identifier: AGPL-3.0-or-later

package beta

import "testing"

func TestEnabled_FailsClosedWithoutAGate(t *testing.T) {
	SetGate(nil)
	if Enabled() {
		t.Fatal("beta reported on with no gate installed; it must fail closed")
	}
}

func TestEnabled_FollowsTheGateOnEveryCall(t *testing.T) {
	t.Cleanup(func() { SetGate(nil) })
	on := false
	SetGate(func() bool { return on })

	if Enabled() {
		t.Fatal("beta on while the gate says off")
	}
	on = true
	if !Enabled() {
		t.Fatal("beta still off after the gate flipped on (the answer must not be cached)")
	}
	on = false
	if Enabled() {
		t.Fatal("beta still on after the gate flipped off")
	}
}
