// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os/signal"
	"syscall"
	"time"
)

// JavascriptISOString format for json time
const JavascriptISOString = "2006-01-02T15:04:05.999Z07:00"

// HTTPRequestJSON struct for storing http request details
type HTTPRequestJSON struct {
	Time             string              `json:"Time"`
	Method           string              `json:"Method"`
	Path             string              `json:"Path"`
	Proto            string              `json:"Proto"`
	ProtoMajor       int                 `json:"ProtoMajor"`
	ProtoMinor       int                 `json:"ProtoMinor"`
	ContentLength    int64               `json:"ContentLength"`
	TransferEncoding []string            `json:"TransferEncoding"`
	Host             string              `json:"Host"`
	RemoteAddr       string              `json:"RemoteAddr"`
	RequestURI       string              `json:"RequestURI"`
	Header           map[string][]string `json:"Header"`
	Form             map[string][]string `json:"Form"`
	PostForm         map[string][]string `json:"PostForm"`
	MultipartForm    *multipart.Form     `json:"MultipartForm"`
	Body             []byte              `json:"Body"`
}

// NewHTTPRequestJSON copy a http request to struct for storing http request details
func NewHTTPRequestJSON(r *http.Request) *HTTPRequestJSON {

	bodyBytes, err := ioutil.ReadAll(r.Body)
	//fmt.Printf("httpReadRequest bodyBytes len: %d\n", len(bodyBytes))

	if err != nil {
		bodyBytes = []byte(fmt.Sprintf("Error reading body: %v", err))
	}

	request := &HTTPRequestJSON{
		Time:             time.Now().UTC().Format(JavascriptISOString),
		Method:           r.Method,
		Path:              r.URL.Path,
		Proto:            r.Proto,
		ProtoMajor:       r.ProtoMajor,
		ProtoMinor:       r.ProtoMinor,
		ContentLength:    r.ContentLength,
		TransferEncoding: r.TransferEncoding,
		Host:             r.Host,
		RemoteAddr:       r.RemoteAddr,
		RequestURI:       r.RequestURI,
		Header:           r.Header,
		Form:             r.Form,
		PostForm:         r.PostForm,
		MultipartForm:    r.MultipartForm,
		Body:             bodyBytes,
	}

	return request

}

// ByteCountDecimal return a human-readable form of the number of bytes
func ByteCountDecimal(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

func writeJSON(w http.ResponseWriter, v interface{}) (bytesWritten int, err error) {
	data, _ := json.Marshal(v)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	bytesWritten, err = w.Write(data)
	return
}

// LoopForever on signal processing
func LoopForever(shutdownHook func()) {
	fmt.Printf("Entering infinite loop\n")

	signal.Notify(OsSignal, syscall.SIGINT, syscall.SIGTERM) // , syscall.SIGUSR1
	_ = <-OsSignal

	fmt.Printf("Exiting infinite loop received OsSignal\n")
	if shutdownHook != nil {
		shutdownHook()
	}

}
