package main

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func ErrToExitCodo(err error) int {
	if err == nil {
		return 0
	}
	if exiterr, ok := err.(*exec.ExitError); ok {
		// The program has exited with an exit code != 0

		// This works on both Unix and Windows. Although package
		// syscall is generally platform dependent, WaitStatus is
		// defined for both Unix and Windows and in both cases has
		// an ExitStatus() method with the same signature.
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	return 127
}

func main() {
	cmd := exec.Command("adb", os.Args[1:]...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		if err := cmd.Start(); err != nil {
			log.Fatal(err)
		}
	}
	os.Exit(ErrToExitCodo(cmd.Wait()))
}
