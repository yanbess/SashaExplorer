// Package lucyshell opens PowerShell in a folder in LucyShell, the terminal of this
// set of programs. The request goes to the standard input of the started process,
// which is the format LucyShell reads.
package lucyshell

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const exeName = "LucyShell.exe"

// startWait is how long Open watches the process. LucyShell ends right after
// passing a request to a running window, and otherwise keeps running as the window.
const startWait = 3 * time.Second

// Errors returned by Open.
var (
	ErrNotFound = errors.New("LucyShell.exe was not found. Put it next to SashaExplorer.exe")
	ErrFailed   = errors.New("LucyShell could not open the folder")
)

type request struct {
	Kind  string `json:"kind"`
	Title string `json:"title"`
	Dir   string `json:"dir"`
}

// Locate finds LucyShell for a program in dir: next to it, or in the build folder
// of the neighboring project during development.
func Locate(dir string) (string, error) {
	candidates := []string{
		filepath.Join(dir, exeName),
		filepath.Join(dir, "..", "..", "..", "LucyShell", "build", "bin", exeName),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return filepath.Clean(candidate), nil
		}
	}
	return "", ErrNotFound
}

// Open starts LucyShell at exe with a shell in folder.
func Open(exe, folder string) error {
	name := filepath.Base(folder)
	if name == "." || name == `\` || name == "" {
		name = folder
	}
	payload, err := json.Marshal(request{Kind: "shell", Title: name, Dir: folder})
	if err != nil {
		return err
	}

	command := exec.Command(exe, "--handoff")
	command.Dir = filepath.Dir(exe)
	input, err := command.StdinPipe()
	if err != nil {
		return fmt.Errorf("start LucyShell: %w", err)
	}
	if err := command.Start(); err != nil {
		return fmt.Errorf("start LucyShell: %w", err)
	}
	finished := make(chan error, 1)
	go func() { finished <- command.Wait() }()

	_, writeErr := input.Write(payload)
	closeErr := input.Close()
	if writeErr != nil || closeErr != nil {
		return ErrFailed
	}
	select {
	case err := <-finished:
		if err != nil {
			return ErrFailed
		}
	case <-time.After(startWait):
	}
	return nil
}
