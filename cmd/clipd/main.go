package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"

	"github.com/trypsynth/clipd/shared"
)

var cfg *shared.Config

func main() {
	var err error
	cfg, err = shared.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	serverAddress := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	if len(os.Args) > 1 && os.Args[1] == "run" {
		if len(os.Args) < 3 {
			log.Fatal("Usage: clipd run <program> [args...]")
		}
		program := shared.ResolvePath(os.Args[2], cfg.DriveMappings)
		args := []string{}
		if len(os.Args) > 3 {
			for _, arg := range os.Args[3:] {
				args = append(args, shared.ResolvePath(arg, cfg.DriveMappings))
			}
		}
		cwd, err := os.Getwd()
		if err != nil {
			log.Fatal("Failed to get current working directory: ", err)
		}
		workingDir := shared.ResolvePath(cwd, cfg.DriveMappings)
		if err := sendRunRequest(serverAddress, program, args, workingDir); err != nil {
			log.Fatal(err)
		}
		return
	}
	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}
	if err := sendClipboardRequest(serverAddress, string(inputData)); err != nil {
		log.Fatal(err)
	}
}

func sendClipboardRequest(address, data string) error {
	request := shared.Request{
		Type:     shared.RequestTypeClipboard,
		Data:     data,
		Password: cfg.Password,
	}
	return sendRequest(address, request)
}

func sendRunRequest(address, program string, args []string, workingDir string) error {
	request := shared.Request{
		Type:       shared.RequestTypeRun,
		Data:       program,
		Args:       args,
		WorkingDir: workingDir,
		Password:   cfg.Password,
	}
	return sendRequest(address, request)
}

func sendRequest(address string, request shared.Request) error {
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
