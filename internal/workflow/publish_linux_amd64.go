package workflow

import (
	"syscall"
	"unsafe"
)

// Linux x86_64 renameat2(RENAME_NOREPLACE) atomically refuses any existing target,
// including empty directories and symlinks. The supported profile is Linux amd64.
func publishDirectory(source, target string) error {
	old, err := syscall.BytePtrFromString(source)
	if err != nil {
		return err
	}
	next, err := syscall.BytePtrFromString(target)
	if err != nil {
		return err
	}
	const renameat2 = 316 // Linux x86_64 asm/unistd_64.h __NR_renameat2.
	const atFDCWD = ^uintptr(99)
	_, _, errno := syscall.Syscall6(renameat2, atFDCWD, uintptr(unsafe.Pointer(old)), atFDCWD, uintptr(unsafe.Pointer(next)), 1, 0)
	if errno != 0 {
		return errno
	}
	return nil
}
