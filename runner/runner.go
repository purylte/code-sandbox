package main

import (
	"bytes"
	"code-sandbox/types"
	"encoding/json"
	"flag"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"time"
)

var mode = flag.String("mode", "server", "Run in \"server\" mode that manages runner or \"runner\" mode that builds untrusted binary in a container.")
var listenAddr = flag.String("listen", ":8080", "Specify HTTP server listen address")

var containerReady chan *types.Container

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

func startContainer() (container *types.Container, err error) {
	name := "runner_" + randStr(10)
	cmd := exec.Command("docker", "run",
		"--name="+name,
		"--rm",
		"--tmpfs=/tmpfs:exec",
		"-i",
		"--runtime=runsc",
		"--network=none",
		"runner",
		"--mode=runner",
	)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	if err = cmd.Start(); err != nil {
		return nil, err
	}

	container = &types.Container{
		Name:   name,
		Stdin:  stdin,
		Stdout: stdout,
		Stderr: stderr,
		Cmd:    cmd,
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

func runBin() {
	binPath := "/tmpfs/untrustedBin.bin"
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

func main() {
	flag.Parse()
	if *mode == "runner" {
		runBin()
		return
	}

	containerReady = make(chan *types.Container)

	mux := http.NewServeMux()
	mux.HandleFunc("/run", runHandler)

	makeWorkers()

	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

func runHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "expected a POST", http.StatusBadRequest)
		return
	}
	c := <-containerReady
	bin, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = c.Stdin.Write(bin)
	if err != nil {
		http.Error(w, "writing to container stdin: "+err.Error(), http.StatusInternalServerError)
		return
	}
	c.Stdin.Close()

	if err = c.Cmd.Wait(); err != nil {
		http.Error(w, "container cmd.Wait(): "+err.Error(), http.StatusInternalServerError)
		return
	}

	stdout := c.Stdout.Bytes()
	stderr := c.Stderr.Bytes()

	b, err := json.Marshal(types.StdOutput{Stdout: string(stdout), Stderr: string(stderr)})
	if err != nil {
		http.Error(w, "output to json marshall: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)
}
