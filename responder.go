// Copyright 2021 Alex Jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
)

// AutoResponses struct to store map of rules and ordered list to be chcked when requests come in
type AutoResponses struct {
	m map[string]*AutoResponse
	l []*AutoResponse
}

var autoResponders *AutoResponses

// Sort sorts the internal list based on Index
func (r *AutoResponses) Sort() []*AutoResponse {
	sort.Slice(r.l, func(i, j int) bool {
		return r.l[i].Index < r.l[j].Index
	})

	return r.l
}

// Save attempts to save the AutoResponses or an error
func (r *AutoResponses) Save() error {

	r.l = make([]*AutoResponse, 0)

	for _, v := range r.m {
		autoResponders.l = append(autoResponders.l, v)
	}
	r.Sort()

	var err error
	for _, val := range r.m {
		err = StoreResponder(val)
		if err != nil {
			return err
		}
	}
	return nil
}

// Size the number of rules
func (r *AutoResponses) Size() int {
	return len(r.m)
}

// Get the AutoResponse from the list of AutoResponders by name
func (r *AutoResponses) Get(name string) (*AutoResponse, bool) {
	val, ok := r.m[name]
	return val, ok
}

// Delete delete the AutoResponse from the list of AutoResponders
func (r *AutoResponses) Delete(name string) bool {
	delete(r.m, name)
	r.Save()
	return true
}

// Update update an AutoResponse
func (r *AutoResponses) Update(payload *AutoResponse) error {
	response, ok := r.Get(payload.Name)
	if !ok {
		return fmt.Errorf("unable to find autoresponder %s", payload.Name)
	}

	response.Name = payload.Name
	response.Index = payload.Index
	response.Method = payload.Method
	response.Path = payload.Path
	response.StatusCode = payload.StatusCode
	response.ContentType = payload.ContentType
	response.Response = payload.Response
	response.ResponseHeaders = payload.ResponseHeaders
	response.Init()
	r.m[response.Name] = response
	return r.Save()
}

// Insert insert new AutoResponse
func (r *AutoResponses) Insert(payload *AutoResponse) error {
	_, ok := r.Get(payload.Name)
	if ok {
		return fmt.Errorf("responder [%s] already exists", payload.Name)
	}

	response := &AutoResponse{}
	response.Index = payload.Index
	response.Name = payload.Name
	response.Method = payload.Method
	response.Path = payload.Path
	response.StatusCode = payload.StatusCode
	response.ContentType = payload.ContentType
	response.Response = payload.Response
	response.ResponseHeaders = payload.ResponseHeaders

	response.Init()
	r.m[response.Name] = response
	return r.Save()
}

// AutoResponse struct to store a AutoResponse
type AutoResponse struct {
	Index           int               `yaml:"index" json:"index"`
	Method          string            `yaml:"method" json:"method"`
	Name            string            `yaml:"name" json:"name"`
	Path            string            `yaml:"path" json:"path"`
	StatusCode      int               `yaml:"status_code" json:"status_code"`
	ContentType     string            `yaml:"content_type" json:"content_type"`
	Response        string            `yaml:"response" json:"response"`
	ResponseHeaders map[string]string `yaml:"responseHeaders" json:"response_headers"`
	pathRegex       *regexp.Regexp    `yaml:"-" json:"-"`
}

// Bytes returns the bytes of the json formatted of the AutoResponse
func (r *AutoResponse) Bytes() []byte {
	dump, _ := json.MarshalIndent(r, "", "    ")
	return dump
}

// Bytes returns the string of the json formatted of the AutoResponse
func (r *AutoResponse) String() string {
	dump, _ := json.MarshalIndent(r, "", "    ")
	return string(dump)
}

// InitializeAutoResponders attempts to load the list of autoresponders from the file responders.yaml
func InitializeAutoResponders() error {
	list, err := LoadResponders()
	if err != nil {
		fmt.Printf("Error Loading AutoResponders: %v\n", err)
		return err
	}

	autoResponders = &AutoResponses{
		m: list,
		l: make([]*AutoResponse, 0),
	}
	for _, v := range list {
		autoResponders.l = append(autoResponders.l, v)
	}
	autoResponders.Sort()
	fmt.Printf("Loaded AutoResponders\n")
	return err
}

// Init initialize struct
func (r *AutoResponse) Init() {

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
		r.Name = "My Rule"
	}

	pathRegex, err := regexp.Compile(r.Path)
	if err == nil {
		r.pathRegex = pathRegex
	}

	if r.ResponseHeaders == nil {
		r.ResponseHeaders = make(map[string]string)
	}
}

// Find attempts to find a match if the AutoResponse to the http.Request
func (r *AutoResponses) Find(req *http.Request) *AutoResponse {
	if autoResponders == nil {
		return nil
	}

	for _, r := range r.l {
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

// CreateDefaultRules create slice of default rules to populate bucket with on first run
func CreateDefaultRules() []*AutoResponse {

	responders := make([]*AutoResponse, 0)
	var responder *AutoResponse

	responder = &AutoResponse{Index: 10, Name: "Rule 1", Method: "GET", Path: "/hello.text", StatusCode: 200, ContentType: "text/plain", Response: `Hello World!!!!`}
	responder.Init()
	responders = append(responders, responder)

	responder = &AutoResponse{Index: 20, Name: "Rule 2", Method: "GET", Path: "/hello.world", StatusCode: 200, ContentType: "text/plain", Response: `thiis is the response of multi
	lines and will see
	how it comes back`}
	responder.Init()
	responder.ResponseHeaders["X-TestResponse"] = "test"

	responders = append(responders, responder)

	responder = &AutoResponse{Index: 30, Name: "Rule 3", Method: "GET", Path: "/hello.json", StatusCode: 200, ContentType: "text/json", Response: `{
	"message": "Hello World"
	}`}
	responder.Init()
	responders = append(responders, responder)

	responder = &AutoResponse{Index: 40, Name: "Rule 4", Method: "POST", Path: "/.*", StatusCode: 200, ContentType: "text/plain", Response: `Hello World!!!!`}
	responder.Init()
	responders = append(responders, responder)

	responder = &AutoResponse{Index: 50, Name: "Rule 5", Method: ".*", Path: "/api/test", StatusCode: 200, ContentType: "text/json", Response: `{
	"ForceQuery": false,
	"RawQuery": "",
	"Fragment": "",
	"RawFragment": ""
	}`}
	responder.Init()
	responders = append(responders, responder)
	return responders
}
