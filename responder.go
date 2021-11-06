// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"regexp"
)

// AutoResponses struct to store the list ofAutoResponse
type AutoResponses struct {
	List []*AutoResponse `yaml:"responses"`
}

// AutoResponse struct to store a AutoResponse
type AutoResponse struct {
	ID          int            `yaml:"id"`
	Method      string         `yaml:"method"`
	Name        string         `yaml:"name"`
	Path        string         `yaml:"path"`
	StatusCode  int            `yaml:"status_code"`
	ContentType string         `yaml:"content_type"`
	Response    string         `yaml:"response"`
	pathRegex   *regexp.Regexp `yaml:"-"`
}

var (
	autoResponders *AutoResponses
)

// InitializeAutoResponders attempts to load the list of autoresponders from the file responders.yaml
func InitializeAutoResponders() error {
	if *responsesFile == "" {
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("ERROR", err)
		return err
	}

	autoResponders, err = LoadAutoResponders()
	if err != nil {
		fmt.Printf("Error Loading AutoResponders: %v\n", err)
		return err
	}
	// out of the box file system notify, that can watch a single file, or a single directory
	if err := watcher.Add(*responsesFile); err != nil {
		fmt.Printf("FileWatcher add error: %v\n", err)
		return err
	}

	go func() {

		defer func() {
			_ = watcher.Close()
		}()

		for {
			select {
			// watch for events
			case event := <-watcher.Events:
				fmt.Printf("Reloading AutoResponders file: %s\n", event.Name)
				autoResponders, err = LoadAutoResponders()
				if err != nil {
					fmt.Printf("Error Loading AutoResponders: %v\n", err)
				}
				break

				// watch for errors
			case err := <-watcher.Errors:
				fmt.Printf("FileWatcher error: %v\n", err)
				break
			}
		}
	}()
	return err
}

// GetAutoResponse get the AutoResponse from the list of AutoResponders
func GetAutoResponse(i int) *AutoResponse {
	if i > -0 && i < len(autoResponders.List) {
		return autoResponders.List[i]
	}
	return nil
}

// FindAutoResponse attempts to find a match if the AutoResponse to the http.Request
func FindAutoResponse(req *http.Request) *AutoResponse {
	if autoResponders == nil {
		return nil
	}

	for _, r := range autoResponders.List {
		matchedMethod, _ := regexp.MatchString(r.Method, req.Method)
		matchedURI := false
		if r.pathRegex != nil {
			matchedURI = r.pathRegex.MatchString(req.RequestURI)
		}

		//fmt.Printf("req.Method: %s req.RequestURI: %s Method: %s Path: %s matchedMethod: %v matchedURI: %v\n", req.Method, req.RequestURI, r.Method, r.Path, matchedMethod, matchedURI)
		if matchedMethod && matchedURI {
			return r
		}
	}
	return nil
}

// LoadAutoResponders attempts to load the AutoResponses or an error
func LoadAutoResponders() (*AutoResponses, error) {

	if *responsesFile != "" {
		responsesContent, err := ioutil.ReadFile(*responsesFile)
		if err != nil {
			return nil, err
		}

		if responsesContent != nil {
			responses := &AutoResponses{}
			validResponses := &AutoResponses{}
			validResponses.List = make([]*AutoResponse, 0)

			err = yaml.Unmarshal(responsesContent, responses)
			if err == nil {
				if responses.List == nil {
					return nil, fmt.Errorf("responses.List is nil")
				}
				//fmt.Printf("len responses.List: %d\n", len(responses.List))

				for i, r := range responses.List {
					if r == nil {
						continue
					}

					if r.Method == "" {
						r.Method = "GET"
					}
					if r.StatusCode == 0 {
						r.StatusCode = 200
					}
					if r.ContentType == "" {
						r.ContentType = "text/plain"
					}
					if r.Path == "" {
						r.Path = "/.*"
					}
					if r.Response == "" {
						r.ContentType = "OK"
					}

					if r.Name == "" {
						r.Name = fmt.Sprintf("Rule %d", i)
					}

					r.ID = i
					pathRegex, err := regexp.Compile(r.Path)
					if err == nil {
						r.pathRegex = pathRegex
					}
					validResponses.List = append(validResponses.List, r)
				}
			}

			if validResponses != nil {
				fmt.Printf("Loaded %d AutoResponders\n", len(validResponses.List))

				fmt.Printf("    Method    Path\n")
				fmt.Printf("--------------------------------------------\n")
				for i, responder := range validResponses.List {
					fmt.Printf("[%d] %-8s %s\n", i, responder.Method, responder.Path)
				}
			}

			return validResponses, err
		}
		return nil, fmt.Errorf("%s is empty", *responsesFile)
	}
	return nil, nil
}
