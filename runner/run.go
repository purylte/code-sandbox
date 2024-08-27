package main

import (
	"io"
	"log"
	"math/rand"
	"os"
	"os/exec"
)

func main() {
	binPath := "/tmpfs/untrustedBin"
	bin, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading stdin: %v", err)
	}

	if err := os.WriteFile(binPath, bin, 0755); err != nil {
		log.Fatalf("writing binary: %v", err)
	}
	defer os.Remove(binPath)

	cmd := exec.Command(binPath)
	cmd.Stderr = os.Stderr
	cmd.Stdout = os.Stdout
	if err := cmd.Run(); err != nil {
		log.Fatalf("cmd.Run(): %v", err)
	}

}

type Container struct {
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

func startContainer() (container *Container, err error) {
	name := "runner_" + randStr(10)
	cmd := exec.Command("docker", "run",
		"--name="+name,
		"--rm",
		"--tmpfs=/tmpfs:exec",
		"-i",
		"--runtime=runsc",
		"--network=none",
		"runner",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdout.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	container = &Container{
		name:   name,
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		cmd:    cmd,
	}

	if err = cmd.Wait(); err != nil {
		return nil, err
	}

	return container, nil

}

func randStr(length int) string {
	var letters = []rune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789")

	s := make([]rune, length)
	for i := range s {
		s[i] = letters[rand.Intn(len(letters))]
	}
	return string(s)
}
