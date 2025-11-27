package main

import (
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"github.com/trypsynth/clipd/clipd"
)

var cfg *clipd.Config

func main() {
	var err error
	cfg, err = clipd.LoadConfig()
	if err != nil {
		log.Fatal(err)
	}
	rootCmd := &cobra.Command{
		Use:   "clipd",
		Short: "Send clipboard data and run programs on Windows from Linux",
		Long:  "clipd is a client for sending clipboard data and executing programs on a Windows machine from a Linux environment.",
		RunE:  clipboardCmd,
	}
	pathCmd := &cobra.Command{
		Use:   "path <path>",
		Short: "Resolve and print a Windows path",
		Args:  cobra.ExactArgs(1),
		RunE:  pathCmdFunc,
	}
	runCmd := &cobra.Command{
		Use:   "run <program> [args...]",
		Short: "Run a program on the Windows machine",
		Args:  cobra.MinimumNArgs(1),
		RunE:  runCmdFunc,
	}
	pipeCmd := &cobra.Command{
		Use:   "pipe <program> [args...]",
		Short: "Pipe stdin to a program on the Windows machine",
		Args:  cobra.MinimumNArgs(1),
		RunE:  pipeCmdFunc,
	}
	rootCmd.AddCommand(pathCmd, runCmd, pipeCmd)
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func clipboardCmd(cmd *cobra.Command, args []string) error {
	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}
	serverAddress := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	return clipd.SendClipboardRequest(serverAddress, string(inputData), cfg.Password)
}

func pathCmdFunc(cmd *cobra.Command, args []string) error {
	path := clipd.ResolvePath(args[0], cfg.DriveMappings)
	fmt.Println(path)
	return nil
}

func runCmdFunc(cmd *cobra.Command, args []string) error {
	program := clipd.ResolvePath(args[0], cfg.DriveMappings)
	cmdArgs := clipd.ResolveArgs(args[1:], cfg.DriveMappings)
	workingDir, err := clipd.GetWorkingDir(cfg.DriveMappings)
	if err != nil {
		return err
	}
	serverAddress := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	return clipd.SendRunRequest(serverAddress, program, cmdArgs, workingDir, cfg.Password)
}

func pipeCmdFunc(cmd *cobra.Command, args []string) error {
	program := clipd.ResolvePath(args[0], cfg.DriveMappings)
	cmdArgs := clipd.ResolveArgs(args[1:], cfg.DriveMappings)
	workingDir, err := clipd.GetWorkingDir(cfg.DriveMappings)
	if err != nil {
		return err
	}
	inputData, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to read stdin: %w", err)
	}
	serverAddress := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.ServerPort)
	return clipd.SendPipeRequest(serverAddress, program, cmdArgs, workingDir, string(inputData), cfg.Password)
}
