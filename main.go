package main

import (
	"flag"
	"fmt"
	"gopkg.in/fsnotify.v1"
	"io/ioutil"
	"log"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/omeid/livereload"
	"github.com/russross/blackfriday"
)

const (
	template = `
<!DOCTYPE html>
<html>
<head>
<meta charset="UTF-8">
<title>%s</title>
<link rel="stylesheet" href="/_assets/sanitize.css" media="all">
<link rel="stylesheet" href="/_assets/github-markdown.css" media="all">
<link rel="stylesheet" href="/_assets/sons-of-obsidian.css" media="all">
<link rel="stylesheet" href="/_assets/style.css" media="all">
<script src="/_assets/jquery-2.1.1.min.js"></script>
<script src="/_assets/prettify.min.js"></script>
<script>
$(function() {
	$('pre>code').each(function() { $(this.parentNode).addClass('prettyprint') }); prettyPrint();
	$.getScript(window.location.protocol + '//' + window.location.hostname + ':35729/livereload.js');
});
</script>
</head>
<body>
<div class="markdown-body">%s</div>
</body>
</html>
`
	extensions = blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS
)

var (
	addr = flag.String("http", ":8000", "HTTP service address (e.g., ':8000')")
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	flag.Parse()
	cwd, _ := os.Getwd()

	lrs := livereload.New("mkup")
	defer lrs.Close()

	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/livereload.js", func(w http.ResponseWriter, r *http.Request) {
			b, err := Asset("_assets/livereload.js")
			if err != nil {
				http.Error(w, "404 page not found", 404)
				return
			}
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(b)
			return
		})
		mux.Handle("/", lrs)
		log.Fatal(http.ListenAndServe(":35729", mux))
	}()

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
			b, err := Asset(name[1:])
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
		b, err := ioutil.ReadFile(filepath.Join(cwd, name))
		if err != nil {
			if os.IsNotExist(err) {
				http.Error(w, "404 page not found", 404)
				return
			}
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderer := blackfriday.HtmlRenderer(0, "", "")
		b = blackfriday.Markdown(b, renderer, extensions)
		w.Write([]byte(fmt.Sprintf(template, name, string(b))))
	})

	server := &http.Server{
		Addr: *addr,
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL.RequestURI())
			http.DefaultServeMux.ServeHTTP(w, r)
		}),
	}

	fmt.Fprintln(os.Stderr, "Lisning at "+*addr)
	log.Fatal(server.ListenAndServe())
}
