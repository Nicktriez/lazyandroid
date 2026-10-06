package android

import (
	"reflect"
	"testing"
)

func TestHasFlag(t *testing.T) {
	args := []string{"install", "--apks=a.apk", "--device=SERIAL"}
	for _, tc := range []struct {
		names []string
		want  bool
	}{
		{[]string{"--apks"}, true},
		{[]string{"--device"}, true},
		{[]string{"--annotate", "--device"}, true},
		{[]string{"--missing"}, false},
	} {
		if got := HasFlag(args, tc.names...); got != tc.want {
			t.Errorf("HasFlag(%q, %q) = %v, want %v", args, tc.names, got, tc.want)
		}
	}
	if HasFlag(args, "--apk") {
		t.Error("--apk matched --apks=; prefix matching must require the '=' form")
	}
}

func TestWithDevice(t *testing.T) {
	got := WithDevice([]string{"install", "--apks=a.apk"}, "SERIAL")
	want := []string{"install", "--apks=a.apk", "--device=SERIAL"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("WithDevice = %q, want %q", got, want)
	}

	// A serial the caller chose by hand is never overridden.
	got = WithDevice([]string{"install", "--device=OTHER"}, "SERIAL")
	if !reflect.DeepEqual(got, []string{"install", "--device=OTHER"}) {
		t.Errorf("WithDevice overrode an explicit device: %q", got)
	}

	// No serial means nothing to add.
	got = WithDevice([]string{"install"}, "")
	if !reflect.DeepEqual(got, []string{"install"}) {
		t.Errorf("WithDevice with no serial changed args: %q", got)
	}
}
