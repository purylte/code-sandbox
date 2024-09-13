package main

import (
	"bytes"
	"code-sandbox/templates"
	"code-sandbox/types"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/a-h/templ"
)

//go:embed static
var static embed.FS

var (
	listenAddr  = flag.String("listen", ":8080", "Specify HTTP server listen address")
	builderAddr = flag.String("builder", "http://localhost:8081/build", "Specify HTTP builder server address")
	runnerAddr  = flag.String("runner", "http://localhost:8082/run", "Specify HTTP runner server address")
)

func main() {
	flag.Parse()
	mux := http.NewServeMux()
	mux.Handle("/static/", http.FileServer(http.FS(static)))
	mux.Handle("/go", templ.Handler(templates.MainLayout(templates.CodeSandbox("go"))))
	mux.Handle("/cpp", templ.Handler(templates.MainLayout(templates.CodeSandbox("cpp"))))
	mux.HandleFunc("/run", handleRun)
	mux.Handle("/", http.NotFoundHandler())

	log.Fatal(http.ListenAndServe(*listenAddr, mux))
}

func handleRun(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "expected a POST", http.StatusBadRequest)
		return
	}
	var lang types.Language
	switch r.FormValue("lang") {
	case "go":
		lang = types.Go
	case "cpp":
		lang = types.CPP
	default:
		http.Error(w, "language "+r.FormValue("lang")+" not supported", http.StatusUnprocessableEntity)
		return
	}

	code := r.FormValue("code")
	bin, err := sendBuildReq(code, lang)
	if err != nil {
		http.Error(w, "build: "+err.Error(), http.StatusInternalServerError)
		return
	}

	stdout, stderr, err := sendRunReq(bin)
	if err != nil {
		http.Error(w, "run: "+err.Error(), http.StatusInternalServerError)
		return
	}
	templates.Output(string(stdout), string(stderr)).Render(r.Context(), w)

}

func sendBuildReq(code string, lang types.Language) (bin []byte, err error) {
	var pathLang string
	switch lang {
	case types.Go:
		pathLang = "/go"
	case types.CPP:
		pathLang = "/cpp"
	default:
		return nil, fmt.Errorf("build language not valid")
	}

	res, err := http.Post(*builderAddr+pathLang, "text/plain", strings.NewReader(code))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil,
			fmt.Errorf("build HTTP request failed with status code: %d", res.StatusCode)
	}

	bin, err = io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	defer res.Body.Close()
	return bin, nil
}

func sendRunReq(bin []byte) (stdout []byte, stderr []byte, err error) {
	bodyReader := bytes.NewReader(bin)
	res, err := http.Post(*runnerAddr, "application/octet-stream", bodyReader)
	if err != nil {
		return nil, nil, err
	}
	if res.StatusCode != http.StatusOK {
		txt, _ := io.ReadAll(res.Body)
		return nil, nil,
			fmt.Errorf("run request failed: %d, %v", res.StatusCode, string(txt))
	}
	var stdOutput types.StdOutput
	if err = json.NewDecoder(res.Body).Decode(&stdOutput); err != nil {
		return nil, nil, err
	}
	defer res.Body.Close()

	return []byte(stdOutput.Stdout), []byte(stdOutput.Stderr), nil
}
