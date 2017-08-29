package main

import (
	"os"
	"io"
	"fmt"
	"net/http"
	"log"
	"flag"
	"strconv"
	"github.com/gorilla/mux"
)

/*
GOOS=linux GOARCH=amd64 go build -o web.linux
GOOS=windows GOARCH=amd64 go build -o web.exe
go build -o web.mac
*/

// DownloadSubHandler : called from FileServiceHandler to perform downloading 
func DownloadSubHandler(path string, w http.ResponseWriter, r *http.Request) {
	_, err := os.Stat(path)
	if os.IsNotExist(err) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "404 error, file not found!\n")
		return
	}
	http.ServeFile(w, r, path)
}

// ChunkedUploadSubHandler : called from FileServiceHandler to perform Chunked transfer
// Actually we don't need to distinguish chunked and non-chunked here, go http lib will
// take care of that for us
func ChunkedUploadSubHandler(path string, w http.ResponseWriter, r *http.Request) {
	out, err := os.Create(path)
	defer out.Close()
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "404 error, file cannot be created!")
	}
	io.Copy(out, r.Body)
	r.Body.Close()
}

// FileServiceHandler : handler for /upload  w.(http.Flusher).Flush()
func FileServiceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := "./upload/" + vars["file"]
	if r.Method == "GET" {
		DownloadSubHandler(path, w, r)
	} else if r.Method == "POST" {
		ChunkedUploadSubHandler(path, w, r)
	} else {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "404 error, page not found!")
	}
}

// ShowWorkingHandler : simply show some text that web server is running 
func ShowWorkingHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Go web server is running!");
}

func main() {
	port := flag.Int("p", 8080, "Port to serve on")
	ssl := flag.Bool("ssl", false, "Use ssl")
	flag.Parse()

	// Make sure upload directory exist
	if _, err := os.Stat("./upload"); os.IsNotExist(err) {
		os.Mkdir("./upload", 0755)
	}

	r := mux.NewRouter()
	r.HandleFunc("/upload/{file}", FileServiceHandler)
	r.HandleFunc("/", ShowWorkingHandler)
	http.Handle("/", r)

	var err error
	if *ssl {
		log.Printf("HTTPS Web server starts up, serving on port: %d", *port)
		err = http.ListenAndServeTLS(":" + strconv.Itoa(*port), "ssl.crt", "ssl.key", nil)
	} else {
		log.Printf("HTTP Web server starts up, serving on port: %d", *port)
		err = http.ListenAndServe(":" + strconv.Itoa(*port), nil)
	}

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}