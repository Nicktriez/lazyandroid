package android

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatalf("read testdata/%s: %v", name, err)
	}
	return string(b)
}

// The fixture is verbatim `android emulator list --long` output; it pins the
// format the fixed-width parser depends on.
func TestParseEmulatorListRealOutput(t *testing.T) {
	emus, err := ParseEmulatorList(readTestdata(t, "emulator_list_long.txt"))
	if err != nil {
		t.Fatalf("ParseEmulatorList: %v", err)
	}
	if len(emus) != 6 {
		t.Fatalf("parsed %d emulators, want 6", len(emus))
	}
	for i, e := range emus {
		if e.ID == "" || e.Name == "" || e.APILevel == "" || e.Status == "" {
			t.Errorf("row %d incomplete: %+v", i, e)
		}
		if strings.ContainsAny(e.ID, " \t") {
			t.Errorf("row %d ID %q contains whitespace: columns are misaligned", i, e.ID)
		}
		if e.Running() {
			t.Errorf("row %d reported running, but every fixture AVD is offline", i)
		}
	}
	if got, want := emus[0], (Emulator{ID: "medium_phone", Name: "Medium Phone", APILevel: "android-36", Status: "Offline"}); got != want {
		t.Errorf("row 0 = %+v, want %+v", got, want)
	}
	if emus[5].Name != "Medium Tablet" {
		t.Errorf("row 5 name = %q, want %q: multi-word names must survive column slicing", emus[5].Name, "Medium Tablet")
	}
}

func TestParseEmulatorListEdges(t *testing.T) {
	const header = "AVD ID                   AVD Name                      API Level      Status         Serial"

	t.Run("empty output", func(t *testing.T) {
		emus, err := ParseEmulatorList("")
		if err != nil {
			t.Fatalf("ParseEmulatorList: %v", err)
		}
		if len(emus) != 0 {
			t.Fatalf("parsed %d emulators from empty output, want 0", len(emus))
		}
	})

	t.Run("no header", func(t *testing.T) {
		if _, err := ParseEmulatorList("Error: no devices found\n"); err == nil {
			t.Fatal("want error when the header row is absent")
		}
	})

	t.Run("row shorter than header", func(t *testing.T) {
		emus, err := ParseEmulatorList(header + "\nmedium_phone             Medium Phone\n")
		if err != nil {
			t.Fatalf("ParseEmulatorList: %v", err)
		}
		want := []Emulator{{ID: "medium_phone", Name: "Medium Phone"}}
		if !reflect.DeepEqual(emus, want) {
			t.Errorf("got %+v, want %+v", emus, want)
		}
	})

	t.Run("serial marks a running AVD", func(t *testing.T) {
		// Build the row from the header offsets so the Serial column lands
		// exactly where the parser expects it. Rows may run past the header
		// width once the Serial column is populated.
		row := []byte(strings.Repeat(" ", 100))
		copy(row[0:], "medium_phone")
		copy(row[25:], "Medium Phone")
		copy(row[55:], "android-36")
		copy(row[70:], "Online")
		copy(row[85:], "emulator-5554")
		emus, err := ParseEmulatorList(header + "\n" + string(row) + "\n")
		if err != nil {
			t.Fatalf("ParseEmulatorList: %v", err)
		}
		if len(emus) != 1 || !emus[0].Running() || emus[0].Serial != "emulator-5554" {
			t.Fatalf("got %+v, want one running AVD with serial emulator-5554", emus)
		}
	})
}

func TestParseInfo(t *testing.T) {
	out := "sdk: /home/nick/Android/Sdk\n" +
		"this line has no colon\n" +
		"version: 1.0.16261425\n" +
		"cache: C:\\dir:with:colons\n"
	want := []KeyValue{
		{Key: "sdk", Value: "/home/nick/Android/Sdk"},
		{Key: "version", Value: "1.0.16261425"},
		{Key: "cache", Value: `C:\dir:with:colons`},
	}
	if got := ParseInfo(out); !reflect.DeepEqual(got, want) {
		t.Errorf("ParseInfo = %+v, want %+v", got, want)
	}
}
