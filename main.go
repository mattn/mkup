package main

import (
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/russross/blackfriday"
)

const (
	template = `
<link rel="stylesheet" href="/_assets/sanitize.css" media="all">
<link rel="stylesheet" href="/_assets/github-markdown.css" media="all">
<link rel="stylesheet" href="/_assets/sons-of-obsidian.css" media="all">
<link rel="stylesheet" href="/_assets/style.css" media="all">
<script src="/_assets/jquery-2.1.1.min.js"></script>
<script src="/_assets/prettify.min.js"></script>
<script>$(function() { $('pre>code').each(function() { $(this.parentNode).addClass('prettyprint') }); prettyPrint(); });</script>
<div class="markdown-body">%s</div>
`
	extensions = blackfriday.EXTENSION_NO_INTRA_EMPHASIS |
		blackfriday.EXTENSION_TABLES |
		blackfriday.EXTENSION_FENCED_CODE |
		blackfriday.EXTENSION_AUTOLINK |
		blackfriday.EXTENSION_STRIKETHROUGH |
		blackfriday.EXTENSION_SPACE_HEADERS
)

func main() {
	cwd, _ := os.Getwd()
	fs := http.FileServer(http.Dir(cwd))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := r.URL.Path
		if strings.HasPrefix(name, "/_assets/") {
			name = name[1:]
			b, err := Asset(name)
			if err != nil {
				http.Error(w, err.Error(), 404)
				return
			}

			w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(name)))
			w.Write(b)
			return
		}
		if filepath.Ext(name) != ".md" {
			fs.ServeHTTP(w, r)
			return
		}
		b, err := ioutil.ReadFile(filepath.Join(cwd, name))
		if err != nil {
			http.Error(w, err.Error(), 403)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		renderer := blackfriday.HtmlRenderer(0, "", "")
		b = blackfriday.Markdown(b, renderer, extensions)
		w.Write([]byte(fmt.Sprintf(template, string(b))))
		//w.Write(blackfriday.MarkdownCommon(b))
	})
	http.ListenAndServe(":8000", nil)
}
