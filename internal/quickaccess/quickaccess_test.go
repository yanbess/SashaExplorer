package quickaccess

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

type fakeShell struct {
	pinned  []string
	failPin string
	calls   []string
}

func (f *fakeShell) Pinned() ([]string, error) { return append([]string(nil), f.pinned...), nil }

func (f *fakeShell) Pin(path string) error {
	f.calls = append(f.calls, "pin "+path)
	if path == f.failPin {
		return errors.New("offline")
	}
	f.pinned = append(f.pinned, path)
	return nil
}

func (f *fakeShell) Unpin(path string) error {
	f.calls = append(f.calls, "unpin "+path)
	for i, p := range f.pinned {
		if strings.EqualFold(p, path) {
			f.pinned = append(f.pinned[:i], f.pinned[i+1:]...)
			break
		}
	}
	return nil
}

func TestSyncPinsNewFoldersAndSkipsDrives(t *testing.T) {
	shell := &fakeShell{pinned: []string{`C:\dev`}}
	got, err := Sync(shell, Record{}, []string{`C:\`, `D:\Photos`, `c:\dev\`, `D:\photos`})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{`pin D:\Photos`}; !reflect.DeepEqual(shell.calls, want) {
		t.Fatalf("calls = %v, want %v", shell.calls, want)
	}
	if want := (Record{Handled: []string{`D:\Photos`, `c:\dev\`}, Pinned: []string{`D:\Photos`}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("record = %+v, want %+v", got, want)
	}
}

func TestSyncUnpinsOnlyItsOwn(t *testing.T) {
	shell := &fakeShell{pinned: []string{`C:\dev`, `D:\Photos`}}
	record := Record{Handled: []string{`D:\Photos`, `C:\dev`}, Pinned: []string{`D:\Photos`}}
	got, err := Sync(shell, record, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{`unpin D:\Photos`}; !reflect.DeepEqual(shell.calls, want) {
		t.Fatalf("calls = %v, want %v", shell.calls, want)
	}
	if len(got.Handled) != 0 || len(got.Pinned) != 0 {
		t.Fatalf("record = %+v, want empty", got)
	}
	if want := []string{`C:\dev`}; !reflect.DeepEqual(shell.pinned, want) {
		t.Fatalf("pinned = %v, want %v", shell.pinned, want)
	}
}

func TestSyncRespectsUnpinInExplorer(t *testing.T) {
	shell := &fakeShell{}
	record := Record{Handled: []string{`D:\Photos`}, Pinned: []string{`D:\Photos`}}
	got, err := Sync(shell, record, []string{`D:\Photos`})
	if err != nil {
		t.Fatal(err)
	}
	if len(shell.calls) != 0 {
		t.Fatalf("calls = %v, want none", shell.calls)
	}
	if !reflect.DeepEqual(got, record) {
		t.Fatalf("record = %+v, want %+v", got, record)
	}
}

func TestSyncRetriesFailedPin(t *testing.T) {
	shell := &fakeShell{failPin: `\\nas\share`}
	got, err := Sync(shell, Record{}, []string{`\\nas\share`, `D:\Photos`})
	if err == nil {
		t.Fatal("want the failed pin reported")
	}
	if want := []string{`D:\Photos`}; !reflect.DeepEqual(got.Handled, want) {
		t.Fatalf("handled = %v, want %v", got.Handled, want)
	}
	shell.failPin = ""
	got, err = Sync(shell, got, []string{`\\nas\share`, `D:\Photos`})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{`\\nas\share`, `D:\Photos`}; !reflect.DeepEqual(got.Pinned, want) {
		t.Fatalf("pinned = %v, want %v", got.Pinned, want)
	}
}

// Turning the setting on again forgets what was dealt with but keeps what was
// pinned, so a folder unpinned in Explorer comes back and stays the sync's.
func TestSyncAfterTurningOnAgain(t *testing.T) {
	shell := &fakeShell{pinned: []string{`D:\Music`}}
	record := Record{Pinned: []string{`D:\Music`, `D:\Photos`}}
	got, err := Sync(shell, record, []string{`D:\Music`, `D:\Photos`})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{`pin D:\Photos`}; !reflect.DeepEqual(shell.calls, want) {
		t.Fatalf("calls = %v, want %v", shell.calls, want)
	}
	if want := []string{`D:\Music`, `D:\Photos`}; !reflect.DeepEqual(got.Pinned, want) {
		t.Fatalf("pinned = %v, want %v", got.Pinned, want)
	}
}

func TestSyncKeepsFailedUnpin(t *testing.T) {
	shell := &failingUnpin{fakeShell{pinned: []string{`D:\Photos`}}}
	record := Record{Handled: []string{`D:\Photos`}, Pinned: []string{`D:\Photos`}}
	got, err := Sync(shell, record, nil)
	if err == nil {
		t.Fatal("want the failed unpin reported")
	}
	if !reflect.DeepEqual(got, record) {
		t.Fatalf("record = %+v, want %+v", got, record)
	}
}

type failingUnpin struct{ fakeShell }

func (f *failingUnpin) Unpin(string) error { return errors.New("busy") }

func TestSyncWithNothingSkipsShell(t *testing.T) {
	got, err := Sync(nilShell{}, Record{}, nil)
	if err != nil || len(got.Handled) != 0 {
		t.Fatalf("got %+v, %v", got, err)
	}
}

type nilShell struct{}

func (nilShell) Pinned() ([]string, error) { return nil, errors.New("must not be called") }
func (nilShell) Pin(string) error          { return errors.New("must not be called") }
func (nilShell) Unpin(string) error        { return errors.New("must not be called") }
