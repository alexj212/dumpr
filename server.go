// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"time"
)

// SpawnTCPListener spawn a tcp listener on the host, port will exit if unable to open port.
func SpawnTCPListener(host string, port int) error {
	fmt.Printf("spawn: %s:%d\n", host, port)
	// Listen for incoming connections.
	listener := fmt.Sprintf("%s:%d", host, port)
	l, err := net.Listen("tcp", listener)
	if err != nil {
		fmt.Println("Error listening:", err.Error())
		return err
	}

	go func() {
		// Close the listener when the application closes.
		defer func() {
			_ = l.Close()
		}()
		conns := clientConns(l)
		for {
			go handleConn(<-conns)
		}
	}()
	return nil
}

func clientConns(listener net.Listener) chan net.Conn {

	ch := make(chan net.Conn)
	i := 0
	go func() {
		for {
			client, err := listener.Accept()
			if client == nil {
				fmt.Printf("couldn't accept: %v\n", err)
				continue
			}
			i++

			//fmt.Printf("%d: %v <-> %v\n", i, client.LocalAddr(), client.RemoteAddr())
			ch <- client
		}
	}()
	return ch
}

func handleConn(client net.Conn) {
	var ip string
	if addr, ok := client.RemoteAddr().(*net.TCPAddr); ok {
		ip = addr.IP.String()
	} else {
		ip = client.RemoteAddr().String()
	}
	session, err := createSession(ip)
	if err != nil {
		_, _ = client.Write([]byte(fmt.Sprintf("unable to create save file - %v", err)))
		_ = client.Close()
		return
	}

	buf := make([]byte, 255)
	b := bufio.NewReader(client)
	checkedForHTTP := false
	sentHeader := false
	fileSize := 0
	for {
		if !checkedForHTTP && b.Buffered() >= 0 {
			//fmt.Printf("2: %d\n", b.Buffered())
			pay, err := b.Peek(16)
			if err == nil {
				//fmt.Printf("len(pay: %d\n", len(pay))
				isHTTPRequest, _ := regexp.Match("^(GET|POST|PUT|DELETE)\\s+.+", pay)
				//fmt.Printf("isHTTPRequest: %v err: %v\n", isHTTPRequest, e)
				if isHTTPRequest {
					//fmt.Printf("3: %d\n", b.Buffered())
					req, err := http.ReadRequest(b)
					//fmt.Printf("4: %d\n", b.Buffered())
					if err != nil {
						break
					}
					session.InitializeHTTP(req)
					checkedForHTTP = true

					response := &http.Response{}
					response.StatusCode = 200
					response.Proto = "HTTP/1.1"
					response.ProtoMajor = 1
					response.ProtoMinor = 1
					response.Request = req
					response.Header = http.Header{}
					response.Header["X-Session-Key"] = []string{session.Key}
					url := fmt.Sprintf("%s/v/%s", *publicUrl, session.Key)
					response.Header["X-Session-URL"] = []string{url}
					url = fmt.Sprintf("%s/api/info/%s", *publicUrl, session.Key)
					response.Header["X-Session-Info-URL"] = []string{url}
					response.Header["Content-Type"] = []string{"application/json; charset=utf-8"}
					s := time.Now().UTC().Format(http.TimeFormat)
					response.Header["Date"] = []string{s}

					autoResponse := autoResponders.Find(req)
					if autoResponse != nil {
						session.HandledByRule = autoResponse.Name
					}

					if autoResponse != nil {
						response.StatusCode = autoResponse.StatusCode
						response.Header["Content-Type"] = []string{autoResponse.ContentType}
						response.Header["X-AutoResponder-Name"] = []string{autoResponse.Name}

						if autoResponse.ResponseHeaders != nil {
							for k, v := range autoResponse.ResponseHeaders {
								response.Header[k] = []string{v}
							}
						}

						payload := []byte(autoResponse.Response)
						response.Body = io.NopCloser(bytes.NewReader(payload))
						response.ContentLength = int64(len(payload))
					} else {
						sessionInfo := createNewSessionResponse(session)
						dump, _ := json.MarshalIndent(sessionInfo, "", "    ")
						response.Body = io.NopCloser(bytes.NewReader(dump))
						response.ContentLength = int64(len(dump))
					}

					var b bytes.Buffer
					w := bufio.NewWriter(&b)
					_ = response.Write(w)
					_ = w.Flush()
					_, _ = client.Write(b.Bytes())
				}
			}
		}

		if checkedForHTTP {
			break
		}

		if !sentHeader {
			Broadcast(SessionUpdated, session.ToApiSession())
			_, _ = client.Write([]byte(fmt.Sprintf("view at %s/v/%s", *publicUrl, session.Key)))
			sentHeader = true
		}

		byteRead, err := b.Read(buf)
		if err != nil { // EOF, or worse
			break
		}
		pay := buf[:byteRead]
		fileSize += len(pay)
		_, _ = session.outputFile.Write(pay)
		_ = m.BroadcastMultiple(pay, session.Viewers)
		if fileSize >= maxSessionSize {
			fmt.Printf("Shuting down session: %s max session size reached: %d maxSessionSize: %d\n", session.Key, fileSize, maxSessionSize)
			break
		}

	}

	deactivateSession(session)
	_ = client.Close()
}
