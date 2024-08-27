package main

import (
	"code-sandbox/templates"
	"embed"
	"log"
	"net/http"

	"github.com/a-h/templ"
)

//go:embed static
var static embed.FS

func main() {
	component := templates.MainLayout(templates.CodeSandbox("go"))
	mux := http.NewServeMux()
	mux.Handle("/go", templ.Handler(component))
	mux.Handle("/static/", http.FileServer(http.FS(static)))
	mux.Handle("/", http.NotFoundHandler())

	log.Fatal(http.ListenAndServe(":8080", mux))
}
