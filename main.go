package main

import (
	"embed"
	"flag"
	"fmt"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"gopkg.in/fsnotify.v1"

	"github.com/omeid/livereload"
	"github.com/russross/blackfriday/v2"
)

const name = "mkup"

const version = "0.0.3"

var revision = "HEAD"

const (
	template = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>%s</title>
<link rel="stylesheet" href="/_assets/sanitize.css" media="all">
<link rel="stylesheet" href="/_assets/style.css" media="all">
<link rel="stylesheet" href="/_assets/github-dark.css" media="all">
<script src="/_assets/highlight.min.js"></script>
<script>hljs.highlightAll();</script>
<script>document.write('<script src="'
	+ location.protocol + '//'
    + (location.host || 'localhost')
    + '%s/_assets/livereload.js?snipver=1"></'
    + 'script>')</script>
</head>
<body>
<div class="markdown-body">%s</div>
</body>
</html>
`
	extensions = blackfriday.NoIntraEmphasis |
		blackfriday.Tables |
		blackfriday.FencedCode |
		blackfriday.Autolink |
		blackfriday.Strikethrough |
		blackfriday.SpaceHeadings
)

var (
	addr        = flag.String("http", ":8000", "HTTP service address (e.g., ':8000')")
	usehttpport = flag.Bool("usehttpport", false, "use livereload port with the same http port")
)

//go:embed _assets
var local embed.FS

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	cwd, _ := os.Getwd()
	livereloadPortAddr := ":35729"
	if *usehttpport == true {
		livereloadPortAddr = ""
	}

	lrs := livereload.New("mkup")
	defer lrs.Close()

	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	go func() {
		fsw.Add(cwd)
		err = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
			if info == nil {
				return err
			}
			if !info.IsDir() {
				return nil
			}
			fsw.Add(path)
			return nil
		})

		for {
			select {
			case event := <-fsw.Events:
				if path, err := filepath.Rel(cwd, event.Name); err == nil {
					path = "/" + filepath.ToSlash(path)
					log.Println("reload", path)
					lrs.Reload(path, true)
				}
			case err := <-fsw.Errors:
				if err != nil {
					log.Println(err)
				}
			}
		}
	}()

	fs := http.FileServer(http.Dir(cwd))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path
		if strings.HasPrefix(name, "/_assets/") {
			b, err := local.ReadFile(name[1:])
			if err != nil {
				http.Error(w, "404 page not found", 404)
				return
			}

			w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
			w.Write(b)
			return
		}
		ext := filepath.Ext(name)
		if ext != ".md" && ext != ".mkd" && ext != ".markdown" {
			fs.ServeHTTP(w, r)
			return
		}
		b, err := os.ReadFile(filepath.Join(cwd, name))
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "404 page not found", 404)
				return
			}
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderer := blackfriday.NewHTMLRenderer(blackfriday.HTMLRendererParameters{})
		b = blackfriday.Run(
			b,
			blackfriday.WithRenderer(renderer),
			blackfriday.WithExtensions(extensions),
		)
		w.Write([]byte(fmt.Sprintf(template, name, livereloadPortAddr, string(b))))
	})
	http.Handle("/livereload",
		http.HandlerFunc(
			func(w http.ResponseWriter, r *http.Request) {
				lrs.ServeHTTP(w, r)
			}))

	server := &http.Server{
		Addr: *addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.RequestURI())
			http.DefaultServeMux.ServeHTTP(w, r)
		}),
	}

	if livereloadPortAddr != "" {
		go func() {
			server := &http.Server{
				Addr: livereloadPortAddr,
				Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.RequestURI())
					http.DefaultServeMux.ServeHTTP(w, r)
				}),
			}
			fmt.Fprintln(os.Stderr, "Listening at "+livereloadPortAddr)
			server.ListenAndServe()
		}()
	}

	fmt.Fprintln(os.Stderr, "Listening at "+*addr)
	log.Fatal(server.ListenAndServe())
}
