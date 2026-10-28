package main

import "os/exec"

func startCommand(name string, args ...string) error {
	return exec.Command(name, args...).Start()
}
