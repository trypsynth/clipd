//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"golang.org/x/sys/windows"
	"log"
	"net"
	"os"
	"syscall"
	"unsafe"
)

type config struct {
	ServerIP   string
	ServerPort int
}

type clipboardRequest struct {
	Data string
}

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
)

func main() {
	f, err := os.Open("config.json")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	var cfg config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		log.Fatal(err)
	}
	addr := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()
	log.Printf("Listening on %s...\n", addr)
	for {
		if conn, err := ln.Accept(); err == nil {
			go handle(conn)
		} else {
			log.Println("Accept error:", err)
		}
	}
}

func handle(c net.Conn) {
	defer c.Close()
	var req clipboardRequest
	if err := json.NewDecoder(c).Decode(&req); err != nil {
		log.Println("Decode error:", err)
		return
	}
	if err := setClipboard(req.Data); err != nil {
		log.Println("Clipboard error:", err)
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
