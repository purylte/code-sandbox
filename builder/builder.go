package main

import (
	"bytes"
	"code-sandbox/types"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

var (
	mode       = flag.String("mode", "server", "Run in \"server\" mode that manages builder or \"builder\" mode that builds untrusted binary in a container.")
	listenAddr = flag.String("listen", ":8080", "Specify HTTP server listen address. Only for in server mode")
	langFlags  = flag.String("langs", "go,cpp", "List seperated comma of languages that will be built. Will ignore unsupported language. Only for in server mode. go,cpp")
	expectLang = flag.String("lang", "", "Expected language that will be built. Only for builder mode")
)

var langMap = map[string]types.Language{
	"go":  types.Go,
	"cpp": types.CPP,
}

var langReadyContainer = map[types.Language]chan *types.Container{
	types.Go:  nil,
	types.CPP: nil,
}

func makeWorkers(langFlagList []string) {
	for _, lang := range langFlagList {
		for i := 0; i < 1; i++ {
			go workerLoop(lang)
		}
	}
}

func workerLoop(langFlag string) {
	for {
		c, err := startContainer(langFlag)
		if err != nil {
			log.Printf("error starting container: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}
		lang := langMap[langFlag]
		langReadyContainer[lang] <- c
	}
}
func startContainer(langFlag string) (container *types.Container, err error) {
	name := fmt.Sprintf("builder_%v_%v", langFlag, randStr(10))
	cmd := exec.Command("docker", "run",
		"--name="+name,
		"--rm",
		"-i",
		"--runtime=runsc",
		"--network=none",
		langFlag+"-builder",
		"--mode=builder",
		"--lang="+langFlag,
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

func genFileExt(lang types.Language) string {
	switch lang {
	case types.Go:
		return ".go"
	case types.CPP:
		return ".cpp"

	default:
		panic(fmt.Sprintf("unhandled language: %v", lang))
	}
}

func genBuildCmd(lang types.Language, srcPath string, outPath string) *exec.Cmd {
	switch lang {
	case types.Go:
		return exec.Command("go", "build", "-o", outPath, srcPath)
	case types.CPP:
		return exec.Command("g++", "--static", srcPath, "-o", outPath)

	default:
		panic(fmt.Sprintf("unhandled language: %v", lang))
	}
}

func runBuild(lang types.Language) {
	src, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatalf("reading stdin: %v", err)
	}

	srcPath := "./src" + genFileExt(lang)
	if err := os.WriteFile(srcPath, src, 0755); err != nil {
		log.Fatalf("writing binary: %v", err)
	}
	defer os.Remove(srcPath)

	outPath := "./out"

	cmd := genBuildCmd(lang, srcPath, outPath)
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		log.Fatalf("cmd.Run(): %v", err)
	}

	file, err := os.Open(outPath)
	if err != nil {
		log.Fatalf("opening file: %v", err)
	}
	defer file.Close()
	defer os.Remove(outPath)

	if _, err := io.Copy(os.Stdout, file); err != nil {
		log.Fatalf("error writing binary file to stdout: %v", err)
	}
}

func main() {
	flag.Parse()
	if *mode == "builder" {
		if *expectLang == "" {
			log.Fatalf("Error: -lang flag is required in builder mode.")
		}
		lang, exist := langMap[*expectLang]
		if !exist {
			log.Fatalf("Language %v is not supported", lang)
		}
		runBuild(lang)
		return
	}

	langFlagList := strings.Split(*langFlags, ",")
	var validLangFlagList []string
	for _, item := range langFlagList {
		if l, exists := langMap[item]; exists {
			validLangFlagList = append(validLangFlagList, item)
			langReadyContainer[l] = make(chan *types.Container)
		} else {
			log.Printf("Language %v is not supported", l)
		}
	}

	makeWorkers(validLangFlagList)

	mux := http.NewServeMux()
	mux.HandleFunc("/build/{lang}", buildHandler)

	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

func buildHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "expected a POST", http.StatusBadRequest)
		return
	}
	lang, exist := langMap[r.PathValue("lang")]
	if !exist {
		http.Error(w, "language "+r.PathValue("lang")+" is not valid", http.StatusBadRequest)
		return
	}

	channel, exist := langReadyContainer[lang]
	if !exist || channel == nil {
		http.Error(w, "language "+r.PathValue("lang")+" build container is not running", http.StatusInternalServerError)
		return
	}
	c := <-channel

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "reading request body: "+err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = c.Stdin.Write(body)
	if err != nil {
		http.Error(w, "writing to container stdin: "+err.Error(), http.StatusInternalServerError)
		return
	}
	c.Stdin.Close()

	if err = c.Cmd.Wait(); err != nil {
		stderr := c.Stderr.Bytes()
		if len(stderr) > 0 {
			http.Error(w, "building binary: "+string(stderr), http.StatusInternalServerError)
			return
		}
		http.Error(w, "container cmd.Wait(): "+err.Error(), http.StatusInternalServerError)
		return
	}

	stdout := c.Stdout.Bytes()
	_, err = w.Write(stdout)
	if err != nil {
		http.Error(w, "writing response: "+err.Error(), http.StatusInternalServerError)
		return
	}
}
