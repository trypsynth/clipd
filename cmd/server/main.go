//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/getlantern/systray"
	"github.com/trypsynth/clipd/clipd"
)

var (
	user32                = windows.NewLazySystemDLL("user32.dll")
	kernel32              = windows.NewLazySystemDLL("kernel32.dll")
	shell32               = windows.NewLazySystemDLL("shell32.dll")
	openClipboard         = user32.NewProc("OpenClipboard")
	emptyClipboard        = user32.NewProc("EmptyClipboard")
	setClipboardData      = user32.NewProc("SetClipboardData")
	closeClipboard        = user32.NewProc("CloseClipboard")
	globalAlloc           = kernel32.NewProc("GlobalAlloc")
	globalLock            = kernel32.NewProc("GlobalLock")
	globalUnlock          = kernel32.NewProc("GlobalUnlock")
	memcpy                = kernel32.NewProc("RtlMoveMemory")
	messageBoxW           = user32.NewProc("MessageBoxW")
	shellExecuteExW       = shell32.NewProc("ShellExecuteExW")
	systemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	waitForInputIdle      = user32.NewProc("WaitForInputIdle")
	cfUnicodeText         = uintptr(13)
	gmemMoveable          = uintptr(2)
	mbIconError           = uintptr(0x00000010)
	server                net.Listener
	serverCtx             context.Context
	serverCancel          context.CancelFunc
	config                *clipd.Config
)

const (
	SEE_MASK_NOCLOSEPROCESS      = 0x00000040
	SW_SHOWNORMAL                = 1
	SPI_GETFOREGROUNDLOCKTIMEOUT = 0x2000
	SPI_SETFOREGROUNDLOCKTIMEOUT = 0x2001
)

type SHELLEXECUTEINFO struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         *uint16
	lpFile         *uint16
	lpParameters   *uint16
	lpDirectory    *uint16
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        *uint16
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       uintptr
}

func showErrorBox(title, message string) {
	titlePtr, err := windows.UTF16PtrFromString(title)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting title: %v\nTitle: %s\nMessage: %s\n", err, title, message)
		return
	}
	messagePtr, err := windows.UTF16PtrFromString(message)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error converting message: %v\nTitle: %s\nMessage: %s\n", err, title, message)
		return
	}
	messageBoxW.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), mbIconError)
}

func main() {
	cfg, err := clipd.LoadConfig()
	if err != nil {
		showErrorBox("Error", fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}
	config = cfg
	serverCtx, serverCancel = context.WithCancel(context.Background())
	go startServer(cfg)
	systray.Run(onReady, onExit)
}

func startServer(cfg *clipd.Config) {
	addr := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		showErrorBox("Error", fmt.Sprintf("Failed to start server on %s: %v", addr, err))
		os.Exit(1)
	}
	defer ln.Close()
	tcpLn := ln.(*net.TCPListener)
	for {
		tcpLn.SetDeadline(time.Now().Add(1 * time.Second))
		conn, err := tcpLn.Accept()
		if serverCtx.Err() != nil {
			return
		}
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			showErrorBox("Error", fmt.Sprintf("Connection accept error: %v", err))
			continue
		}
		go handle(conn)
	}
}

func onReady() {
	systray.SetTitle("Clipd")
	systray.SetTooltip("Clipd Server")
	mQuit := systray.AddMenuItem("Quit", "Quit the server")
	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				systray.Quit()
				return
			}
		}
	}()
}

func onExit() {
	if serverCancel != nil {
		serverCancel()
	}
	os.Exit(0)
}

func handle(c net.Conn) {
	defer c.Close()
	var req clipd.Request
	decoder := json.NewDecoder(c)
	if err := decoder.Decode(&req); err != nil {
		showErrorBox("Clipd Server Error", fmt.Sprintf("Failed to decode request: %v", err))
		return
	}
	if config.Password != req.Password {
		showErrorBox("Clipd Server Error", "Incorrect password received.")
		return
	}
	switch req.Type {
	case clipd.RequestTypeClipboard:
		if err := setClipboard(req.Data); err != nil {
			showErrorBox("Error", fmt.Sprintf("Clipboard operation failed: %v", err))
		}
	case clipd.RequestTypeRun:
		if err := runProgram(req.Data, req.Args, req.WorkingDir); err != nil {
			showErrorBox("Error", fmt.Sprintf("Program execution failed: %v", err))
		}
	case clipd.RequestTypePipe:
		if err := runProgramWithInput(req.Data, req.Args, req.WorkingDir, req.Stdin); err != nil {
			showErrorBox("Error", fmt.Sprintf("Program pipe execution failed: %v", err))
		}
	default:
		showErrorBox("Error", fmt.Sprintf("Unknown request type: %v", req.Type))
	}
}

func setClipboard(s string) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if r, _, err := openClipboard.Call(0); r == 0 {
		return err
	}
	defer closeClipboard.Call()
	emptyClipboard.Call()
	utf16 := syscall.StringToUTF16(s)
	h, _, err := globalAlloc.Call(gmemMoveable, uintptr(len(utf16)*2))
	if h == 0 {
		return err
	}
	p, _, _ := globalLock.Call(h)
	if p == 0 {
		return fmt.Errorf("GlobalLock failed")
	}
	memcpy.Call(p, uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)*2))
	globalUnlock.Call(h)
	if r, _, err := setClipboardData.Call(cfUnicodeText, h); r == 0 {
		return err
	}
	return nil
}

func runProgram(program string, args []string, workingDir string) error {
	lpFile, err := clipd.ToUTF16Ptr(program, "program path")
	if err != nil {
		return err
	}
	lpParameters, err := clipd.OptionalUTF16Ptr(buildArgsString(args), "parameters")
	if err != nil {
		return err
	}
	lpDirectory, err := clipd.OptionalUTF16Ptr(workingDir, "working directory")
	if err != nil {
		return err
	}
	sei := SHELLEXECUTEINFO{
		cbSize:       uint32(unsafe.Sizeof(SHELLEXECUTEINFO{})),
		fMask:        SEE_MASK_NOCLOSEPROCESS,
		lpFile:       lpFile,
		lpParameters: lpParameters,
		lpDirectory:  lpDirectory,
		nShow:        SW_SHOWNORMAL,
	}
	var oldTimeout uintptr
	systemParametersInfoW.Call(SPI_GETFOREGROUNDLOCKTIMEOUT, 0, uintptr(unsafe.Pointer(&oldTimeout)), 0)
	systemParametersInfoW.Call(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, 0, 0)
	ret, _, err := shellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	if ret == 0 {
		systemParametersInfoW.Call(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, oldTimeout, 0)
		return fmt.Errorf("ShellExecuteEx failed: %v", err)
	}
	defer func() {
		if sei.hProcess != 0 {
			windows.CloseHandle(windows.Handle(sei.hProcess))
		}
	}()
	waitForInputIdle.Call(uintptr(sei.hProcess), 5000)
	systemParametersInfoW.Call(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, oldTimeout, 0)
	return nil
}

func runProgramWithInput(program string, args []string, workingDir, stdinData string) error {
	resolvedProgram, err := resolveExecutable(program)
	if err != nil {
		return err
	}
	lpFile, err := clipd.ToUTF16Ptr(resolvedProgram, "program path")
	if err != nil {
		return err
	}
	lpDirectory, err := clipd.OptionalUTF16Ptr(workingDir, "working directory")
	if err != nil {
		return err
	}
	commandLine := buildCommandLine(resolvedProgram, args)
	cmdLine, err := windows.UTF16FromString(commandLine)
	if err != nil {
		return fmt.Errorf("failed to build command line: %w", err)
	}
	sa := inheritableSA()
	var readPipe, writePipe windows.Handle
	if err := windows.CreatePipe(&readPipe, &writePipe, &sa, 0); err != nil {
		return fmt.Errorf("failed to create pipe: %w", err)
	}
	defer closeHandle(&readPipe)
	defer closeHandle(&writePipe)
	if err := windows.SetHandleInformation(writePipe, windows.HANDLE_FLAG_INHERIT, 0); err != nil {
		return fmt.Errorf("failed to configure pipe handle: %w", err)
	}
	stdoutHandle, err := openNullHandle(&sa)
	if err != nil {
		return fmt.Errorf("failed to open NUL for stdout: %w", err)
	}
	defer closeHandle(&stdoutHandle)
	stderrHandle, err := openNullHandle(&sa)
	if err != nil {
		return fmt.Errorf("failed to open NUL for stderr: %w", err)
	}
	defer closeHandle(&stderrHandle)
	startupInfo := &windows.StartupInfo{
		Cb:        uint32(unsafe.Sizeof(windows.StartupInfo{})),
		Flags:     windows.STARTF_USESTDHANDLES,
		StdInput:  readPipe,
		StdOutput: stdoutHandle,
		StdErr:    stderrHandle,
	}
	var procInfo windows.ProcessInformation
	var oldTimeout uintptr
	systemParametersInfoW.Call(SPI_GETFOREGROUNDLOCKTIMEOUT, 0, uintptr(unsafe.Pointer(&oldTimeout)), 0)
	systemParametersInfoW.Call(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, 0, 0)
	err = windows.CreateProcess(lpFile, &cmdLine[0], nil, nil, true, 0, nil, lpDirectory, startupInfo, &procInfo)
	if err != nil {
		systemParametersInfoW.Call(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, oldTimeout, 0)
		return fmt.Errorf("CreateProcess failed: %w", err)
	}
	windows.CloseHandle(procInfo.Thread)
	defer windows.CloseHandle(procInfo.Process)
	windows.CloseHandle(readPipe)
	readPipe = 0
	if err := writeToHandle(writePipe, []byte(stdinData)); err != nil {
		systemParametersInfoW.Call(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, oldTimeout, 0)
		return fmt.Errorf("failed to write stdin: %w", err)
	}
	windows.CloseHandle(writePipe)
	writePipe = 0
	waitForInputIdle.Call(uintptr(procInfo.Process), 5000)
	systemParametersInfoW.Call(SPI_SETFOREGROUNDLOCKTIMEOUT, 0, oldTimeout, 0)
	return nil
}

func writeToHandle(handle windows.Handle, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	for len(data) > 0 {
		var written uint32
		if err := windows.WriteFile(handle, data, &written, nil); err != nil {
			return err
		}
		if written == 0 {
			return fmt.Errorf("no data written to handle")
		}
		data = data[written:]
	}
	return nil
}

func buildCommandLine(program string, args []string) string {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteArgument(program))
	for _, arg := range args {
		parts = append(parts, quoteArgument(arg))
	}
	return strings.Join(parts, " ")
}

// buildArgsString builds a properly quoted argument string from a slice of arguments.
func buildArgsString(args []string) string {
	if len(args) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, arg := range args {
		if i > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(quoteArgument(arg))
	}
	return sb.String()
}

func resolveExecutable(program string) (string, error) {
	if program == "" {
		return "", fmt.Errorf("program path is empty")
	}
	if strings.ContainsAny(program, "\\/:") {
		return program, nil
	}
	resolved, err := exec.LookPath(program)
	if err != nil {
		return "", fmt.Errorf("failed to find %q on PATH: %w", program, err)
	}
	return resolved, nil
}

func quoteArgument(arg string) string {
	return syscall.EscapeArg(arg)
}

func inheritableSA() windows.SecurityAttributes {
	return windows.SecurityAttributes{
		Length:        uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		InheritHandle: 1,
	}
}

func openNullHandle(sa *windows.SecurityAttributes) (windows.Handle, error) {
	handle, err := windows.CreateFile(
		windows.StringToUTF16Ptr("NUL"),
		windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		sa,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return 0, err
	}
	return handle, nil
}

func closeHandle(h *windows.Handle) {
	if h == nil || *h == 0 {
		return
	}
	windows.CloseHandle(*h)
	*h = 0
}
