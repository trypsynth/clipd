package main

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

type Config struct {
	ServerIP   string
	ServerPort int
}

type ClipboardRequest struct {
	Data string
}

func main() {
	configFile, err := os.Open("config.json")
	if err != nil {
		fmt.Println("Error opening config file:", err)
		return
	}
	defer configFile.Close()
	var config Config
	err = json.NewDecoder(configFile).Decode(&config)
	if err != nil {
		fmt.Println("Error decoding config:", err)
		return
	}
	address := fmt.Sprintf("%s:%d", config.ServerIP, config.ServerPort)
	ln, err := net.Listen("tcp", address)
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer ln.Close()
	fmt.Printf("Server listening on %s...\n", address)
	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	var request ClipboardRequest
	if err := json.NewDecoder(conn).Decode(&request); err != nil {
		fmt.Println("Error decoding request:", err)
		return
	}
	if err := setClipboardText(request.Data); err != nil {
		fmt.Println("Error setting clipboard:", err)
	}
}

func setClipboardText(text string) error {
	user32 := windows.NewLazySystemDLL("user32.dll")
	kernel32 := windows.NewLazySystemDLL("kernel32.dll")
	openClipboard := user32.NewProc("OpenClipboard")
	emptyClipboard := user32.NewProc("EmptyClipboard")
	setClipboardData := user32.NewProc("SetClipboardData")
	closeClipboard := user32.NewProc("CloseClipboard")
	globalAlloc := kernel32.NewProc("GlobalAlloc")
	globalLock := kernel32.NewProc("GlobalLock")
	globalUnlock := kernel32.NewProc("GlobalUnlock")
	memcpy := kernel32.NewProc("RtlMoveMemory")
	const (
		CF_UNICODETEXT = 13
		GMEM_MOVEABLE  = 0x0002
	)
	r, _, err := openClipboard.Call(0)
	if r == 0 {
		return fmt.Errorf("OpenClipboard failed: %v", err)
	}
	defer closeClipboard.Call()
	emptyClipboard.Call()
	hMem, _, err := globalAlloc.Call(GMEM_MOVEABLE, uintptr(len(text)*2+2))
	if hMem == 0 {
		return fmt.Errorf("GlobalAlloc failed: %v", err)
	}
	ptr, _, _ := globalLock.Call(hMem)
	if ptr == 0 {
		return fmt.Errorf("GlobalLock failed")
	}
	copyUTF16 := syscall.StringToUTF16(text)
	memcpy.Call(ptr, uintptr(unsafe.Pointer(&copyUTF16[0])), uintptr(len(copyUTF16)*2))
	globalUnlock.Call(hMem)
	r, _, err = setClipboardData.Call(CF_UNICODETEXT, hMem)
	if r == 0 {
		return fmt.Errorf("SetClipboardData failed: %v", err)
	}
	return nil
}
