package files

import (
	"fmt"
	"time"

	"golang.org/x/sys/windows"
)

// Drive is a volume with a letter.
type Drive struct {
	Path  string `json:"path"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
	Total uint64 `json:"total"`
	Free  uint64 `json:"free"`
	Ready bool   `json:"ready"`
	// FileSystem is the format, such as NTFS or FAT32.
	FileSystem string `json:"fileSystem"`
	// Serial is the volume serial number as Windows shows it, such as 1A2B-3C4D.
	Serial string `json:"serial"`
}

// driveWait limits how long a drive may take to answer. A disconnected network
// drive or an empty card reader can take much longer.
const driveWait = 1500 * time.Millisecond

// Drives lists the drives with letters.
func Drives() []Drive {
	mask, err := windows.GetLogicalDrives()
	if err != nil {
		return nil
	}
	var roots []string
	for i := range 26 {
		if mask&(1<<i) != 0 {
			roots = append(roots, string(rune('A'+i))+`:\`)
		}
	}

	results := make([]chan Drive, len(roots))
	for i, root := range roots {
		results[i] = make(chan Drive, 1)
		go func() { results[i] <- readDrive(root) }()
	}
	drives := make([]Drive, 0, len(roots))
	deadline := time.After(driveWait)
	for i, root := range roots {
		select {
		case drive := <-results[i]:
			drives = append(drives, drive)
		case <-deadline:
			drives = append(drives, Drive{Path: root, Kind: driveKind(root)})
		}
	}
	return drives
}

func driveKind(root string) string {
	switch windows.GetDriveType(utf16Ptr(root)) {
	case windows.DRIVE_REMOVABLE:
		return "removable"
	case windows.DRIVE_REMOTE:
		return "network"
	case windows.DRIVE_CDROM:
		return "cd"
	case windows.DRIVE_RAMDISK:
		return "ram"
	default:
		return "fixed"
	}
}

func readDrive(root string) Drive {
	drive := Drive{Path: root, Kind: driveKind(root)}
	var label, fileSystem [windows.MAX_PATH + 1]uint16
	var serial uint32
	if err := windows.GetVolumeInformation(utf16Ptr(root), &label[0], uint32(len(label)), &serial, nil, nil, &fileSystem[0], uint32(len(fileSystem))); err == nil {
		drive.Label = windows.UTF16ToString(label[:])
		drive.FileSystem = windows.UTF16ToString(fileSystem[:])
		drive.Serial = fmt.Sprintf("%04X-%04X", serial>>16, serial&0xFFFF)
		drive.Ready = true
	}
	var free, total, totalFree uint64
	if err := windows.GetDiskFreeSpaceEx(utf16Ptr(root), &free, &total, &totalFree); err == nil {
		drive.Total, drive.Free = total, free
		drive.Ready = true
	}
	return drive
}

func utf16Ptr(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

// KnownFolders returns the paths of the user folders, such as the desktop, by
// name. A folder moved to OneDrive has its new path.
func KnownFolders() map[string]string {
	ids := map[string]*windows.KNOWNFOLDERID{
		"desktop":   windows.FOLDERID_Desktop,
		"downloads": windows.FOLDERID_Downloads,
		"documents": windows.FOLDERID_Documents,
		"pictures":  windows.FOLDERID_Pictures,
		"music":     windows.FOLDERID_Music,
		"videos":    windows.FOLDERID_Videos,
		"home":      windows.FOLDERID_Profile,
	}
	folders := make(map[string]string, len(ids))
	for name, id := range ids {
		if path, err := windows.KnownFolderPath(id, 0); err == nil {
			folders[name] = path
		}
	}
	return folders
}
