package state

import (
	"runtime"
	"testing"
)

// withTempHome points the OS "home directory" lookup at a temp dir so tests
// never touch the real ~/.gator/session.json.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		t.Setenv("USERPROFILE", dir)
	} else {
		t.Setenv("HOME", dir)
	}
	return dir
}

func TestRead_NoFileYet(t *testing.T) {
	withTempHome(t)

	sess, err := Read()
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if sess.CurrentUserName != "" {
		t.Errorf("CurrentUserName = %q, want empty string when no session file exists", sess.CurrentUserName)
	}
}

func TestWriteThenRead_RoundTrip(t *testing.T) {
	withTempHome(t)

	want := Session{CurrentUserName: "alice"}
	if err := Write(want); err != nil {
		t.Fatalf("Write returned error: %v", err)
	}

	got, err := Read()
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if got != want {
		t.Errorf("Read() = %+v, want %+v", got, want)
	}
}

func TestWrite_Overwrites(t *testing.T) {
	withTempHome(t)

	if err := Write(Session{CurrentUserName: "alice"}); err != nil {
		t.Fatalf("first Write returned error: %v", err)
	}
	if err := Write(Session{CurrentUserName: "bob"}); err != nil {
		t.Fatalf("second Write returned error: %v", err)
	}

	got, err := Read()
	if err != nil {
		t.Fatalf("Read returned error: %v", err)
	}
	if got.CurrentUserName != "bob" {
		t.Errorf("CurrentUserName = %q, want %q", got.CurrentUserName, "bob")
	}
}
