package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var mode = flag.String("mode", "server", "Run in \"server\" mode that manages builder or \"builder\" mode that builds untrusted binary in a container.")

type Container struct {
	name   string
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.Reader
	stderr io.Reader
}

var containerReady chan *Container

func makeWorkers() {
	for i := 0; i < 1; i++ {
		go workerLoop()
	}
}

func workerLoop() {
	for {
		c, err := startContainer()
		if err != nil {
			log.Printf("error starting container: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		containerReady <- c
	}
}
func startContainer() (container *Container, err error) {
	name := "builder_" + randStr(10)
	cmd := exec.Command("docker", "run",
		"--name="+name,
		"--rm",
		"--tmpfs=/tmpfs:exec",
		"-i",
		"--runtime=runsc",
		"--network=none",
		"builder",
		"--mode=builder",
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

type Language int

const (
	Go Language = iota
)

var lang = Go

func genFileExt() string {
	switch lang {
	case Go:
		return ".go"

	default:
		panic(fmt.Sprintf("unhandled language: %v", lang))
	}
}

func genBuildCmd(srcPath string, outPath string) *exec.Cmd {
	switch lang {
	case Go:
		return exec.Command("go", "build", "-o", outPath, srcPath)

	default:
		panic(fmt.Sprintf("unhandled language: %v", lang))
	}
}

func runBuild() {
	srcPath := "./src" + genFileExt()

	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading stdin: %v", err)
	}

	if err := os.WriteFile(srcPath, src, 0755); err != nil {
		log.Fatalf("writing binary: %v", err)
	}
	defer os.Remove(srcPath)

	outPath := "./out"

	cmd := genBuildCmd(srcPath, outPath)

	if err := cmd.Run(); err != nil {
		log.Fatalf("cmd.Run(): %v", err)
	}

	file, err := os.Open(outPath)
	if err != nil {
		log.Fatalf("opening file: %v", err)
	}
	defer file.Close()
}

func main() {
	flag.Parse()
	if *mode == "builder" {
		runBuild()
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/builder", buildHandler)

	makeWorkers()

	log.Fatal(http.ListenAndServe(":8080", mux))
}

func buildHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "expected a POST", http.StatusBadRequest)
		return
	}
	c := <-containerReady
	code, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = c.stdin.Write(code)
	if err != nil {
		http.Error(w, "writing to container stdin: "+err.Error(), http.StatusInternalServerError)
		return
	}
	c.stdin.Close()

	output, err := io.ReadAll(c.stdout)
	if err != nil {
		http.Error(w, "reading from container stdout: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = w.Write(output)
	if err != nil {
		http.Error(w, "writing response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err = c.cmd.Wait(); err != nil {
		http.Error(w, "container execution error: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
