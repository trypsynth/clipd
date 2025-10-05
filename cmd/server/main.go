//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/getlantern/systray"
	"github.com/trypsynth/clipd/shared"
)

var (
	user32                   = windows.NewLazySystemDLL("user32.dll")
	kernel32                 = windows.NewLazySystemDLL("kernel32.dll")
	shell32                  = windows.NewLazySystemDLL("shell32.dll")
	openClipboard            = user32.NewProc("OpenClipboard")
	emptyClipboard           = user32.NewProc("EmptyClipboard")
	setClipboardData         = user32.NewProc("SetClipboardData")
	closeClipboard           = user32.NewProc("CloseClipboard")
	globalAlloc              = kernel32.NewProc("GlobalAlloc")
	globalLock               = kernel32.NewProc("GlobalLock")
	globalUnlock             = kernel32.NewProc("GlobalUnlock")
	memcpy                   = kernel32.NewProc("RtlMoveMemory")
	messageBoxW              = user32.NewProc("MessageBoxW")
	shellExecuteExW          = shell32.NewProc("ShellExecuteExW")
	systemParametersInfoW    = user32.NewProc("SystemParametersInfoW")
	getProcessId             = kernel32.NewProc("GetProcessId")
	cfUnicodeText            = uintptr(13)
	gmemMoveable             = uintptr(2)
	mbIconError              = uintptr(0x00000010)
	server                   net.Listener
	serverCtx                context.Context
	serverCancel             context.CancelFunc
	config                   *shared.Config
)

const (
	SEE_MASK_NOCLOSEPROCESS = 0x00000040
	SW_SHOWNORMAL           = 1
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
	titlePtr, _ := windows.UTF16PtrFromString(title)
	messagePtr, _ := windows.UTF16PtrFromString(message)
	messageBoxW.Call(0, uintptr(unsafe.Pointer(messagePtr)), uintptr(unsafe.Pointer(titlePtr)), mbIconError)
}

func main() {
	cfg, err := shared.LoadConfig()
	if err != nil {
		showErrorBox("Error", fmt.Sprintf("Failed to load config: %v", err))
		os.Exit(1)
	}
	config = cfg
	serverCtx, serverCancel = context.WithCancel(context.Background())
	go startServer(cfg)
	systray.Run(onReady, onExit)
}

func startServer(cfg *shared.Config) {
	addr := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		showErrorBox("Error", fmt.Sprintf("Failed to start server on %s: %v", addr, err))
		os.Exit(1)
	}
	server = ln
	defer ln.Close()
	for {
		select {
		case <-serverCtx.Done():
			return
		default:
			if conn, err := ln.Accept(); err == nil {
				go handle(conn)
			} else {
				if serverCtx.Err() != nil {
					return
				}
				showErrorBox("Error", fmt.Sprintf("Connection accept error: %v", err))
			}
		}
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
	if server != nil {
		server.Close()
	}
	os.Exit(0)
}

func handle(c net.Conn) {
	defer c.Close()
	var req shared.Request
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
	case shared.RequestTypeClipboard:
		if err := setClipboard(req.Data); err != nil {
			showErrorBox("Error", fmt.Sprintf("Clipboard operation failed: %v", err))
		}
	case shared.RequestTypeRun:
		if err := runProgram(req.Data, req.Args, req.WorkingDir); err != nil {
			showErrorBox("Error", fmt.Sprintf("Program execution failed: %v", err))
		}
	default:
		showErrorBox("Error", fmt.Sprintf("Unknown request type: %v", req.Type))
	}
}

func setClipboard(s string) error {
	if r, _, err := openClipboard.Call(0); r == 0 {
		return err
	}
	defer closeClipboard.Call()
	emptyClipboard.Call()
	h, _, err := globalAlloc.Call(gmemMoveable, uintptr(len(s)*2+2))
	if h == 0 {
		return err
	}
	p, _, _ := globalLock.Call(h)
	if p == 0 {
		return fmt.Errorf("GlobalLock failed")
	}
	utf16 := syscall.StringToUTF16(s)
	memcpy.Call(p, uintptr(unsafe.Pointer(&utf16[0])), uintptr(len(utf16)*2))
	globalUnlock.Call(h)
	if r, _, err := setClipboardData.Call(cfUnicodeText, h); r == 0 {
		return err
	}
	return nil
}

func runProgram(program string, args []string, workingDir string) error {
	lpFile, err := windows.UTF16PtrFromString(program)
	if err != nil {
		return fmt.Errorf("failed to convert program path: %v", err)
	}
	var lpParameters *uint16
	if len(args) > 0 {
		params := ""
		for i, arg := range args {
			if i > 0 {
				params += " "
			}
			if strings.Contains(arg, " ") {
				params += fmt.Sprintf("\"%s\"", arg)
			} else {
				params += arg
			}
		}
		lpParameters, err = windows.UTF16PtrFromString(params)
		if err != nil {
			return fmt.Errorf("failed to convert parameters: %v", err)
		}
	}
	var lpDirectory *uint16
	if workingDir != "" {
		lpDirectory, err = windows.UTF16PtrFromString(workingDir)
		if err != nil {
			return fmt.Errorf("failed to convert working directory: %v", err)
		}
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
	systemParametersInfoW.Call(0x2000, 0, uintptr(unsafe.Pointer(&oldTimeout)), 0)
	systemParametersInfoW.Call(0x2001, 0, 0, 0)
	ret, _, err := shellExecuteExW.Call(uintptr(unsafe.Pointer(&sei)))
	systemParametersInfoW.Call(0x2001, 0, oldTimeout, 0)
	if ret == 0 {
		return fmt.Errorf("ShellExecuteEx failed: %v", err)
	}
	return nil
}
