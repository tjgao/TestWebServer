package main

import (
	"flag"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

/*
GOOS=linux GOARCH=amd64 go build -o web.linux
GOOS=windows GOARCH=amd64 go build -o web.exe
go build -o web.mac
*/

// DownloadSubHandler : called from FileServiceHandler to perform downloading
func DownloadSubHandler(path string, w http.ResponseWriter, r *http.Request) {
	log.Debug("received a request to download ", path)
	st, err := os.Stat(path)
	if os.IsNotExist(err) || st.Mode().IsDir() {
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
	log.Debug("received a request to upload ", path)
	out, err := os.Create(path)
	defer out.Close()
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		log.Error("failed to create file:", path)
		fmt.Fprintf(w, "404 error, file cannot be created!\n")
		return
	}
	io.Copy(out, r.Body)
	r.Body.Close()
}

func fileExist(path string) bool {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false
	}
	return true
}

// MultishotsUploadSubHandler : called from FileServiceHandler to perform non-standard multishots upload
func MultishotsUploadSubHandler(path string, start int64, end int64, length int64, w http.ResponseWriter, r *http.Request) {
	exist := fileExist(path)
	log.Debug("received a request to do multishot upload: ", path, " range(", start, ", ", end, ")")
	bytesToGo := end - start + 1
	if bytesToGo > length {
		bytesToGo = length
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Error("failed to open file for writting: ", err)
		fmt.Fprintf(w, "failed to open file for writting")
	} else {
		defer f.Close()
		// seek to the start position first
		_, e := f.Seek(start, 0)
		if e != nil {
			w.WriteHeader(http.StatusInternalServerError)
			log.Error("failed to seek in file for writting")
			fmt.Fprintf(w, "failed to seek in file for writting")
		} else {
			if bytesToGo > 0 {
				written, e := io.Copy(f, r.Body)
				if e != nil || written != bytesToGo {
					log.Error("failed to write uploaded data. written (", written, ") required (", bytesToGo, "), ", e)
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "Failed to write uploaded data")
				}
			}
			r.Body.Close()
			if !exist {
				w.WriteHeader(http.StatusCreated)
			}
		}
	}
}

func validateRange(start int64, end int64) bool {
	if end < start || end < 0 || start < 0 {
		return false
	}
	return true
}

// CleanupHandler : handler for /cleanup, all uploaded files will be deleted
// So caller can be sure the environment is clean and there is no interference from previous upload
func CleanupHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("received a request to clean up uploade files")
	uploadPath := "./upload"
	e := filepath.Walk(uploadPath, func(path string, f os.FileInfo, err error) error {
		var ee error
		if uploadPath != path {
			ee = os.Remove(path)
		}
		return ee
	})
	if e != nil {
		log.Error("failed to cleanup uploaded files: ", e)
		fmt.Fprintf(w, "Found error when cleaning up uploaded files")
	}
}

// FileServiceHandler : handler for /upload, two different upload modes are supported.
// 1. stream (chunked transfer), one header with multiple requests, http 1.1 compliant, post and put allowed
// 2. multishots, multiple headers to specify different parts of the uploaded file, non-standard, post and put allowed
func FileServiceHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	path := "./upload/" + vars["file"]
	if r.Method == "GET" {
		DownloadSubHandler(path, w, r)
	} else {
		if r.Method != "PUT" && r.Method != "POST" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			fmt.Fprintf(w, "Not allowed method!")
			log.Error("the method is not allowed")
		} else {
			val := r.Header.Get("Content-Range")
			if val == "" {
				ChunkedUploadSubHandler(path, w, r)
			} else {
				contentLength := r.Header.Get("Content-Length")
				var length int64
				_, er := fmt.Sscanf(strings.TrimSpace(contentLength), "%d", &length)
				var start, end int64
				_, err := fmt.Sscanf(strings.TrimSpace(val), "bytes %d-%d/*", &start, &end)
				if er != nil || err != nil || !validateRange(start, end) || length < 0 {
					log.Error("error or invalid range: ", err)
					w.WriteHeader(http.StatusInternalServerError)
					fmt.Fprintf(w, "Severe internal error!")
				} else {
					MultishotsUploadSubHandler(path, start, end, length, w, r)
				}
			}
		}
	}
}

// ShowWorkingHandler : simply show some text that web server is running
func ShowWorkingHandler(w http.ResponseWriter, r *http.Request) {
	log.Debug("received a request to show status")
	fmt.Fprintf(w, "Go web server is running!")
}

// <program> -h => Show usage
func main() {
	logLevelTable := map[string]log.Level{
		"panic": log.PanicLevel,
		"error": log.ErrorLevel,
		"warn":  log.WarnLevel,
		"info":  log.InfoLevel,
		"debug": log.DebugLevel,
	}

	logLevel := flag.String("l", "info", "pick a log level, available levels are: panic, error, warn, info and debug")
	port := flag.Int("p", 8080, "Port to serve on")
	ssl := flag.Bool("ssl", false, "Use ssl")
	flag.Parse()

	if level, ok := logLevelTable[*logLevel]; ok {
		log.SetLevel(level)
	} else {
		log.Warn("unrecognized log level specified, use warn level instead")
	}

	// Make sure upload directory exist
	if _, err := os.Stat("./upload"); os.IsNotExist(err) {
		os.Mkdir("./upload", 0755)
	}

	r := mux.NewRouter()
	r.HandleFunc("/upload/{file}", FileServiceHandler)
	r.HandleFunc("/", ShowWorkingHandler)
	r.HandleFunc("/cleanup", CleanupHandler)
	http.Handle("/", r)
	http.Handle("/browse/", http.StripPrefix("/browse/", http.FileServer(http.Dir("./upload/"))))

	var err error
	if *ssl {
		log.Info("HTTPS Web server starts up, serving on port: ", *port)
		err = http.ListenAndServeTLS(":"+strconv.Itoa(*port), "ssl.crt", "ssl.key", nil)
	} else {
		log.Info("HTTP Web server starts up, serving on port: ", *port)
		err = http.ListenAndServe(":"+strconv.Itoa(*port), nil)
	}

	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
