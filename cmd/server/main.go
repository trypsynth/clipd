//go:build windows

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"log"
	"net"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"github.com/getlantern/systray"
	"github.com/trypsynth/clipd/shared"
)

var (
	user32           = windows.NewLazySystemDLL("user32.dll")
	kernel32         = windows.NewLazySystemDLL("kernel32.dll")
	openClipboard    = user32.NewProc("OpenClipboard")
	emptyClipboard   = user32.NewProc("EmptyClipboard")
	setClipboardData = user32.NewProc("SetClipboardData")
	closeClipboard   = user32.NewProc("CloseClipboard")
	globalAlloc      = kernel32.NewProc("GlobalAlloc")
	globalLock       = kernel32.NewProc("GlobalLock")
	globalUnlock     = kernel32.NewProc("GlobalUnlock")
	memcpy           = kernel32.NewProc("RtlMoveMemory")
	cfUnicodeText    = uintptr(13)
	gmemMoveable     = uintptr(2)
	server           net.Listener
	serverCtx        context.Context
	serverCancel     context.CancelFunc
)

func main() {
	cfg, err := shared.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	serverCtx, serverCancel = context.WithCancel(context.Background())
	go startServer(cfg)
	systray.Run(onReady, onExit)
}

func startServer(cfg *shared.Config) {
	addr := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	server = ln
	defer ln.Close()
	log.Printf("Listening on %s...\n", addr)
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
				log.Println("Accept error:", err)
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
		log.Println("Decode error:", err)
		return
	}
	switch req.Type {
	case shared.RequestTypeClipboard:
		if err := setClipboard(req.Data); err != nil {
			log.Println("Clipboard error:", err)
		}
	case shared.RequestTypeRun:
		if err := runProgram(req.Program, []string{req.Data}); err != nil {
			log.Printf("Program execution error: %v", err)
		}
	default:
		log.Printf("Unknown request type: %s", req.Type)
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

func runProgram(program string, args []string) error {
	log.Printf("Executing: %s %v", program, args)
	cmd := exec.Command(program, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start program %s: %v", program, err)
	}
	return nil
}
