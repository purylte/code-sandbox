package types

import (
	"bytes"
	"io"
	"os/exec"
)

type Container struct {
	Name   string
	Stdin  io.WriteCloser
	Stdout *bytes.Buffer
	Stderr *bytes.Buffer
	Cmd    *exec.Cmd
}
