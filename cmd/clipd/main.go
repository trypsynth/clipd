package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
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
		log.Fatalf("Error opening config file:", err)
	}
	defer configFile.Close()
	var config Config
	decoder := json.NewDecoder(configFile)
	err = decoder.Decode(&config)
	if err != nil {
		log.Fatalf("Error decoding config file:", err)
	}
	serverAddress := fmt.Sprintf("%s:%d", config.ServerIP, config.ServerPort)
	var inputData bytes.Buffer
	_, err = io.Copy(&inputData, os.Stdin)
	if err != nil {
		log.Fatalf("Error reading input: %v\n", err)
	}
	err = sendToClipboardServer(serverAddress, inputData.String())
	if err != nil {
		log.Fatalf("Error sending data to clipboard server: %v\n", err)
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
	_, err = conn.Write(jsonData)
	if err != nil {
		return fmt.Errorf("error writing to server: %v", err)
	}
	buf := make([]byte, 1024)
	_, err = conn.Read(buf)
	if err != nil && err != io.EOF {
		return fmt.Errorf("error reading response from server: %v", err)
	}
	return nil
}
