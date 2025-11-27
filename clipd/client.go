package clipd

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
)

func SendClipboardRequest(address, data, password string) error {
	request := Request{
		Type:     RequestTypeClipboard,
		Data:     data,
		Password: password,
	}
	return sendRequest(address, request)
}

func SendRunRequest(address, program string, args []string, workingDir, password string) error {
	request := Request{
		Type:       RequestTypeRun,
		Data:       program,
		Args:       args,
		WorkingDir: workingDir,
		Password:   password,
	}
	return sendRequest(address, request)
}

func SendPipeRequest(address, program string, args []string, workingDir, stdin, password string) error {
	request := Request{
		Type:       RequestTypePipe,
		Data:       program,
		Args:       args,
		WorkingDir: workingDir,
		Password:   password,
		Stdin:      stdin,
	}
	return sendRequest(address, request)
}

func sendRequest(address string, request Request) error {
	jsonData, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("error marshalling data: %w", err)
	}
	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer conn.Close()
	if _, err := conn.Write(jsonData); err != nil {
		return fmt.Errorf("error writing to server: %w", err)
	}
	buf := make([]byte, 1024)
	if _, err := conn.Read(buf); err != nil && err != io.EOF {
		return fmt.Errorf("error reading response from server: %w", err)
	}
	return nil
}
