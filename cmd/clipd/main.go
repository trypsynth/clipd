package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
)

type Config struct {
	ServerIP   string
	ServerPort int
}

type ClipboardRequest struct {
	Data string
}

func main() {
	exePath, err := os.Executable()
	if err != nil {
		log.Fatalf("Failed to get executable path: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	configPath := filepath.Join(exeDir, "config.json")
	configFile, err := os.Open(configPath)
	if err != nil {
		log.Fatal(err)
	}
	defer configFile.Close()
	var config Config
	if err := json.NewDecoder(configFile).Decode(&config); err != nil {
		log.Fatal(err)
	}
	serverAddress := fmt.Sprintf("%s:%d", config.ServerIP, config.ServerPort)
	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	if err := sendToClipboardServer(serverAddress, string(inputData)); err != nil {
		log.Fatal(err)
	}
}

func sendToClipboardServer(address, data string) error {
	request := ClipboardRequest{Data: data}
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error marshalling data: %v", err)
	}
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %v", err)
	}
	defer conn.Close()
	if _, err := conn.Write(jsonData); err != nil {
		return fmt.Errorf("error writing to server: %v", err)
	}
	buf := make([]byte, 1024)
	if _, err := conn.Read(buf); err != nil && err != io.EOF {
		return fmt.Errorf("error reading response from server: %v", err)
	}
	return nil
}
