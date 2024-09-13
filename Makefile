dev:
	go run .

templates/*_templ.go: templates/*.templ
	templ generate templates/*.templ

tmp/main: *.go templates/*_templ.go
	go build -o tmp/main

build: tmp/main 

watch-css:
	npx tailwindcss build -o ./static/css/tailwind.css --watch