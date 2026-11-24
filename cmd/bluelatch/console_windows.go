package main

import (
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// detachedEnv is set in the process that is started for the window of a
// terminal. That process never starts another one, whatever its console is.
const detachedEnv = "BLUELATCH_DETACHED"

var (
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
)

// winConsole is the console of Windows.
type winConsole struct{}

// consoleProcesses returns how many processes are attached to the console of
// this process, 0 when there is none.
func consoleProcesses() int {
	// A buffer of two is enough to tell one process from several: when the buffer
	// is too small the call returns the number that is needed.
	var pids [2]uint32
	n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids)))
	return int(n)
}

func (winConsole) State() consoleState {
	if os.Getenv(detachedEnv) != "" {
		return consoleNone
	}
	switch n := consoleProcesses(); {
	case n == 0:
		return consoleNone
	case n == 1:
		return consoleOwn
	default:
		return consoleShared
	}
}

func (winConsole) Free() { procFreeConsole.Call() }

func (winConsole) StartDetached(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), detachedEnv+"=1")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}
