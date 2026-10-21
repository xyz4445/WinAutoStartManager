package schedule

import (
	"encoding/binary"
	"os/user"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestTaskNames(t *testing.T) {
	if got := TaskName(TierAdmin); got != "AutoStartManager-Admin" {
		t.Fatalf("admin task name = %q", got)
	}
	if got := TaskName(TierUser); got != "AutoStartManager-User" {
		t.Fatalf("user task name = %q", got)
	}
	if TierAdmin.Valid() != true || TierUser.Valid() != true || Tier("root").Valid() != false {
		t.Fatal("Tier.Valid is wrong")
	}
}

func TestXMLAdminRunLevel(t *testing.T) {
	got, err := XML(`C:\Tools\app.exe`, TierAdmin)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	for _, want := range []string{
		"<RunLevel>HighestAvailable</RunLevel>",
		"--launch --tier=admin",
		"<Delay>PT2S</Delay>",
		"<Hidden>true</Hidden>",
		"<MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>",
		"<ExecutionTimeLimit>PT5M</ExecutionTimeLimit>",
		"<DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>",
		"<StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>",
		"<LogonType>InteractiveToken</LogonType>",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("XML missing %q\n%s", want, got)
		}
	}
}

func TestXMLUserRunLevel(t *testing.T) {
	got, err := XML(`C:\Tools\app.exe`, TierUser)
	if err != nil {
		t.Fatalf("XML: %v", err)
	}
	if !strings.Contains(got, "<RunLevel>LeastPrivilege</RunLevel>") {
		t.Fatalf("user RunLevel wrong:\n%s", got)
	}
	if !strings.Contains(got, "--launch --tier=user") {
		t.Fatalf("user arguments wrong:\n%s", got)
	}
}

func TestXMLEscapesAmpersand(t *testing.T) {
	got, err := XML(`C:\A&B\x.exe`, TierAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `C:\A&amp;B\x.exe`) {
		t.Fatalf("ampersand not escaped:\n%s", got)
	}
	if strings.Contains(got, "A&B") {
		t.Fatalf("raw ampersand found:\n%s", got)
	}
}

func TestXMLKeepsChineseAndSpaces(t *testing.T) {
	path := `C:\Program Files\工具\rclone.exe`
	got, err := XML(path, TierUser)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, `C:\Program Files\工具\rclone.exe`) {
		t.Fatalf("path not preserved:\n%s", got)
	}
	if !strings.Contains(got, `C:\Program Files\工具`) {
		t.Fatalf("working directory not preserved:\n%s", got)
	}
}

func TestXMLUsesCurrentUser(t *testing.T) {
	got, err := XML(`C:\x.exe`, TierAdmin)
	if err != nil {
		t.Fatal(err)
	}
	u, err := user.Current()
	if err != nil {
		t.Skip("no current user")
	}
	if !strings.Contains(got, u.Username) {
		t.Fatalf("XML does not contain current user %q:\n%s", u.Username, got)
	}
}

func TestXMLRejectsInvalidTier(t *testing.T) {
	if _, err := XML(`C:\x.exe`, Tier("root")); err == nil {
		t.Fatal("expected error for invalid tier")
	}
	if _, err := XML("", TierAdmin); err == nil {
		t.Fatal("expected error for empty exe path")
	}
}

func TestUTF16LEHasBOMAndRoundTrips(t *testing.T) {
	const s = `<?xml version="1.0"?><Task>C:\工具\a.exe</Task>`
	b := UTF16LE(s)
	if len(b) < 2 || b[0] != 0xFF || b[1] != 0xFE {
		t.Fatalf("missing UTF-16LE BOM: % x", b[:2])
	}
	units := make([]uint16, (len(b)-2)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(b[2+i*2:])
	}
	if got := string(utf16.Decode(units)); got != s {
		t.Fatalf("round trip = %q, want %q", got, s)
	}
}
