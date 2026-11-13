// Package terminals finds the terminals and shells on the computer and opens
// one of them in a folder: PowerShell, Command Prompt, Git Bash, the WSL
// distributions, Windows Terminal, LucyShell, and terminals added by hand.
package terminals

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"

	"sashaexplorer/internal/lucyshell"
)

// IDs of the terminals found on the computer. A WSL distribution is
// WSLPrefix followed by its name.
const (
	LucyShell       = "lucyshell"
	PowerShell      = "powershell"
	PowerShell7     = "pwsh"
	CommandPrompt   = "cmd"
	GitBash         = "gitbash"
	WindowsTerminal = "wt"
	WSLPrefix       = "wsl:"
)

// Terminal is a terminal that can open a folder.
type Terminal struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// Custom is set for a terminal added by hand.
	Custom bool   `json:"custom"`
	Path   string `json:"path"`
	// Args are the arguments of a terminal added by hand. {folder} becomes the
	// folder, quoted.
	Args string `json:"args"`
}

// ErrNotFound means the terminal is not on the computer anymore.
var ErrNotFound = errors.New("this terminal was not found on the computer")

// Find lists the terminals on the computer. exeDir is the folder of this
// program, next to which LucyShell may be.
func Find(exeDir string) []Terminal {
	var list []Terminal
	add := func(id, name, path string) {
		if path != "" {
			list = append(list, Terminal{ID: id, Name: name, Path: path})
		}
	}
	if path, err := lucyshell.Locate(exeDir); err == nil {
		add(LucyShell, "LucyShell", path)
	}
	system := systemDir()
	add(PowerShell, "Windows PowerShell", existing(filepath.Join(system, "WindowsPowerShell", "v1.0", "powershell.exe")))
	add(PowerShell7, "PowerShell 7", findPwsh())
	add(CommandPrompt, "Command Prompt", existing(filepath.Join(system, "cmd.exe")))
	add(GitBash, "Git Bash", findGitBash())
	add(WindowsTerminal, "Windows Terminal", findWindowsTerminal())
	if wsl := existing(filepath.Join(system, "wsl.exe")); wsl != "" {
		for _, name := range wslDistributions() {
			add(WSLPrefix+name, name+" (WSL)", wsl)
		}
	}
	return list
}

// Default is the terminal the button opens when none was chosen: LucyShell
// when it is here, or else Windows PowerShell.
func Default(found []Terminal) string {
	for _, terminal := range found {
		if terminal.ID == LucyShell {
			return LucyShell
		}
	}
	return PowerShell
}

// Open opens a terminal in folder. A terminal found on the computer is looked
// up again by its ID, so only its known command line runs.
func Open(terminal Terminal, folder, exeDir string) error {
	if info, err := os.Stat(folder); err != nil || !info.IsDir() {
		return errors.New("the folder cannot be opened in a terminal")
	}
	if terminal.Custom {
		if strings.TrimSpace(terminal.Path) == "" {
			return ErrNotFound
		}
		return run(expandPath(terminal.Path), expandArgs(terminal.Args, folder), folder)
	}
	var found *Terminal
	for _, candidate := range Find(exeDir) {
		if candidate.ID == terminal.ID {
			found = &candidate
			break
		}
	}
	if found == nil {
		return ErrNotFound
	}
	switch {
	case found.ID == LucyShell:
		return lucyshell.Open(found.Path, folder)
	case found.ID == PowerShell:
		return run(found.Path, "-NoExit -NoLogo", folder)
	case found.ID == PowerShell7:
		return run(found.Path, "-NoExit -NoLogo -WorkingDirectory "+syscall.EscapeArg(folder), folder)
	case found.ID == CommandPrompt:
		return run(found.Path, "/K", folder)
	case found.ID == GitBash:
		return run(found.Path, syscall.EscapeArg("--cd="+folder), folder)
	case found.ID == WindowsTerminal:
		return run(found.Path, "-d "+escapeTrailing(folder), folder)
	case strings.HasPrefix(found.ID, WSLPrefix):
		name := strings.TrimPrefix(found.ID, WSLPrefix)
		return run(found.Path, "-d "+syscall.EscapeArg(name)+" --cd "+syscall.EscapeArg(folder), folder)
	}
	return ErrNotFound
}

// run starts a program the way Explorer does, so a console program gets a
// console of its own.
func run(path, args, dir string) error {
	var params *uint16
	if args != "" {
		params = windows.StringToUTF16Ptr(args)
	}
	return windows.ShellExecute(0, windows.StringToUTF16Ptr("open"), windows.StringToUTF16Ptr(path),
		params, windows.StringToUTF16Ptr(dir), windows.SW_SHOWNORMAL)
}

// expandArgs puts the quoted folder in place of {folder}.
func expandArgs(args, folder string) string {
	return strings.ReplaceAll(args, "{folder}", syscall.EscapeArg(folder))
}

// escapeTrailing quotes a folder for Windows Terminal, which reads a root such
// as C:\ badly when its backslash comes before the closing quote.
func escapeTrailing(folder string) string {
	if strings.HasSuffix(folder, `\`) {
		folder += "."
	}
	return syscall.EscapeArg(folder)
}

func expandPath(path string) string {
	path = strings.Trim(strings.TrimSpace(path), `"`)
	if expanded, err := registry.ExpandString(path); err == nil {
		return expanded
	}
	return path
}

func systemDir() string {
	if dir, err := windows.GetSystemDirectory(); err == nil {
		return dir
	}
	return filepath.Join(os.Getenv("SystemRoot"), "System32")
}

func existing(path string) string {
	if info, err := os.Stat(path); err == nil && !info.IsDir() {
		return path
	}
	return ""
}

func findPwsh() string {
	if path, err := exec.LookPath("pwsh.exe"); err == nil {
		return path
	}
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432")} {
		if root == "" {
			continue
		}
		if path := existing(filepath.Join(root, "PowerShell", "7", "pwsh.exe")); path != "" {
			return path
		}
	}
	return ""
}

func findGitBash() string {
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432"), filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs")} {
		if root == "" {
			continue
		}
		if path := existing(filepath.Join(root, "Git", "git-bash.exe")); path != "" {
			return path
		}
	}
	// git.exe is in Git\cmd or Git\bin, next to git-bash.exe one level up.
	if git, err := exec.LookPath("git.exe"); err == nil {
		return existing(filepath.Join(filepath.Dir(filepath.Dir(git)), "git-bash.exe"))
	}
	return ""
}

func findWindowsTerminal() string {
	if path := existing(filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "WindowsApps", "wt.exe")); path != "" {
		return path
	}
	if path, err := exec.LookPath("wt.exe"); err == nil {
		return path
	}
	return ""
}

// wslDistributions reads the installed distributions from the registry, which
// is faster than asking wsl.exe and opens no window.
func wslDistributions() []string {
	key, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Lxss`, registry.READ)
	if err != nil {
		return nil
	}
	defer key.Close()
	ids, err := key.ReadSubKeyNames(-1)
	if err != nil {
		return nil
	}
	var names []string
	for _, id := range ids {
		sub, err := registry.OpenKey(key, id, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		name, _, err := sub.GetStringValue("DistributionName")
		sub.Close()
		// Docker keeps its own distributions, which are not for people.
		if err == nil && name != "" && !strings.HasPrefix(strings.ToLower(name), "docker-desktop") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
