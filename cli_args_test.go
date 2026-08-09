package main

import (
	"reflect"
	"testing"
)

func TestSplitFlagsAndPositional_FlagAfterPositional(t *testing.T) {
	// This is the exact real-world case that broke `memo remote add-device
	// TestPhone --port 19999`: Go's flag package stops parsing at the first
	// non-flag argument, so without this pre-split, "--port"/"19999" would
	// have silently ended up inside fs.Args() instead of being parsed.
	flagArgs, positional := splitFlagsAndPositional([]string{"TestPhone", "--port", "19999"}, nil)
	if !reflect.DeepEqual(flagArgs, []string{"--port", "19999"}) {
		t.Errorf("flagArgs = %v, want [--port 19999]", flagArgs)
	}
	if !reflect.DeepEqual(positional, []string{"TestPhone"}) {
		t.Errorf("positional = %v, want [TestPhone]", positional)
	}
}

func TestSplitFlagsAndPositional_FlagsBeforePositional(t *testing.T) {
	flagArgs, positional := splitFlagsAndPositional([]string{"--port", "19999", "TestPhone"}, nil)
	if !reflect.DeepEqual(flagArgs, []string{"--port", "19999"}) {
		t.Errorf("flagArgs = %v, want [--port 19999]", flagArgs)
	}
	if !reflect.DeepEqual(positional, []string{"TestPhone"}) {
		t.Errorf("positional = %v, want [TestPhone]", positional)
	}
}

func TestSplitFlagsAndPositional_EqualsForm(t *testing.T) {
	flagArgs, positional := splitFlagsAndPositional([]string{"TestPhone", "--port=19999"}, nil)
	if !reflect.DeepEqual(flagArgs, []string{"--port=19999"}) {
		t.Errorf("flagArgs = %v, want [--port=19999]", flagArgs)
	}
	if !reflect.DeepEqual(positional, []string{"TestPhone"}) {
		t.Errorf("positional = %v, want [TestPhone]", positional)
	}
}

func TestSplitFlagsAndPositional_BoolFlagDoesNotConsumeNextToken(t *testing.T) {
	flagArgs, positional := splitFlagsAndPositional([]string{"--lan", "install-arg"}, map[string]bool{"lan": true})
	if !reflect.DeepEqual(flagArgs, []string{"--lan"}) {
		t.Errorf("flagArgs = %v, want [--lan]", flagArgs)
	}
	if !reflect.DeepEqual(positional, []string{"install-arg"}) {
		t.Errorf("positional = %v, want [install-arg] — a bool flag must not swallow the next token", positional)
	}
}

func TestSplitFlagsAndPositional_MultiplePositionals(t *testing.T) {
	flagArgs, positional := splitFlagsAndPositional([]string{"admin", "hunter2", "--port", "8090"}, nil)
	if !reflect.DeepEqual(flagArgs, []string{"--port", "8090"}) {
		t.Errorf("flagArgs = %v, want [--port 8090]", flagArgs)
	}
	if !reflect.DeepEqual(positional, []string{"admin", "hunter2"}) {
		t.Errorf("positional = %v, want [admin hunter2]", positional)
	}
}

func TestSplitFlagsAndPositional_Empty(t *testing.T) {
	flagArgs, positional := splitFlagsAndPositional(nil, nil)
	if len(flagArgs) != 0 || len(positional) != 0 {
		t.Errorf("expected both empty, got flagArgs=%v positional=%v", flagArgs, positional)
	}
}
