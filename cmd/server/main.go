package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
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
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Println("Error decoding config file:", err)
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
	decoder := json.NewDecoder(conn)
	err := decoder.Decode(&request)
	if err != nil {
		fmt.Println("Error decoding request:", err)
		return
	}
	cmd := exec.Command("clip")
	cmd.Stdin = bytes.NewReader([]byte(request.Data))
	err = cmd.Run()
	if err != nil {
		fmt.Println("Error copying to clipboard:", err)
	}
}
