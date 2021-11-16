// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"github.com/dustin/go-humanize"
	"github.com/gin-gonic/gin"
	"github.com/hako/durafmt"
	"gopkg.in/olahol/melody.v1"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// Protocol enum to define protocol various types
type Protocol int64

const (
	// TCP enum to define tcp protocol
	TCP Protocol = 0

	// HTTP enum to define http protocol
	HTTP = 1

	// UNKNOWN enum to define unknown protocol
	UNKNOWN Protocol = 2
)

var (
	// Sessions map of all sessions
	Sessions = make(map[string]*Session)
)

// GetInActiveSessions returns a sorted list of inactive sessions, sorted by age
func GetInActiveSessions() []*ApiSession {
	list := make([]*ApiSession, 0)
	for _, v := range Sessions {
		if !v.Active {
			list = append(list, v.ToApiSession())
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].AgeMs < list[j].AgeMs
	})
	return list
}

// GetActiveSessions returns a sorted list of active sessions, sorted by age
func GetActiveSessions() []*ApiSession {
	list := make([]*ApiSession, 0)
	for _, v := range Sessions {
		if v.Active {
			list = append(list, v.ToApiSession())
		}
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].AgeMs < list[j].AgeMs
	})

	return list
}

// GetAllSessions returns a sorted list of all sessions, sorted by age
func GetAllSessions() []*ApiSession {
	list := make([]*ApiSession, 0)
	for _, v := range Sessions {
		list = append(list, v.ToApiSession())
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].AgeMs < list[j].AgeMs
	})
	return list
}

// String return the human-readable form of Protocol enum
func (s Protocol) String() string {
	switch s {
	case HTTP:
		return "http"
	case TCP:
		return "tcp"
	}
	return "unknown"
}

// MultiPartFile struct to store details of a multipart form upload
type MultiPartFile struct {
	File      string `json:"file"`
	Size      int64  `json:"size"`
	HumanSize string `json:"humanSize"`
}

// Session struct to store details of a session
type Session struct {
	IP             string                    `json:"ip"`
	SaveFile       string                    `json:"file"`
	Key            string                    `json:"key"`
	StartTime      time.Time                 `json:"startTime"`
	EndTime        time.Time                 `json:"endTime"`
	Viewers        []*melody.Session         `json:"-"`
	Protocol       Protocol                  `json:"protocol"`
	MultiPartFiles map[string]*MultiPartFile `json:"multipartFiles"`
	outputFile     *os.File                  `json:"-"`
	Active         bool                      `json:"active"`
	HTTPMethod     string                    `json:"httpMethod"`
	HTTPPath       string                    `json:"httpPath"`
	HandledByRule  string                    `json:"handled_by_rule"`
	HTTPSession    *HTTPRequestJSON          `json:"-"`
}

// ApiSession struct to store details of a session to be returned via web service in json form
type ApiSession struct {
	IP                string                    `json:"ip"`
	Key               string                    `json:"key"`
	StartTime         string                    `json:"startTime"`
	EndTime           string                    `json:"endTime"`
	Protocol          Protocol                  `json:"protocol"`
	MultiPartFiles    map[string]*MultiPartFile `json:"multipartFiles"`
	Active            bool                      `json:"active"`
	HTTPMethod        string                    `json:"httpMethod"`
	HTTPPath          string                    `json:"httpPath"`
	AgeMs             int64                     `json:"ageMs"`
	StartTimeMs       int64                     `json:"startTimeMs"`
	Age               string                    `json:"age"`
	SessionActiveTime string                    `json:"sessionActiveTime"`
	Description       string                    `json:"description"`
	HandledByRule     string                    `json:"handled_by_rule"`
	Size              *SizeResult               `json:"size"`
}

// ToApiSession returns the struct for web consumption of the Session
func (s *Session) ToApiSession() *ApiSession {
	apiSession := &ApiSession{
		IP:                s.IP,
		Key:               s.Key,
		StartTime:         s.FormattedStartTime(),
		EndTime:           s.FormattedEndTime(),
		Protocol:          s.Protocol,
		MultiPartFiles:    s.MultiPartFiles,
		Active:            s.Active,
		HTTPMethod:        s.HTTPMethod,
		HTTPPath:          s.HTTPPath,
		AgeMs:             s.AgeMs(),
		Age:               s.Age(),
		StartTimeMs:       s.StartTime.Unix(),
		SessionActiveTime: s.SessionActiveTime(),
		Description:       s.Description(),
		HandledByRule:     s.HandledByRule,
		Size:              s.Size(),
	}
	return apiSession
}

// AgeMs returns the age in ms of age of the session
func (s *Session) AgeMs() int64 {
	age := time.Now().Sub(s.StartTime)
	return age.Milliseconds()
}

// Age returns the age in human-readable form of age of the session
func (s *Session) Age() string {
	d := time.Now().Sub(s.StartTime)
	duration := durafmt.Parse(d)
	return duration.Format(duraFormatOveride)
}

// SessionActiveTime returns the duration of session in human-readable format
func (s *Session) SessionActiveTime() string {
	d := s.EndTime.Sub(s.StartTime)
	duration := durafmt.Parse(d)
	return duration.Format(duraFormatOveride)
}

// FormattedStartTime returns formatted start time
func (s *Session) FormattedStartTime() string {
	return s.StartTime.Format(time.ANSIC)
}

// FormattedEndTime returns formatted end time
func (s *Session) FormattedEndTime() string {
	return s.EndTime.Format(time.ANSIC)
}

// Description returns the description of a session for web
func (s *Session) Description() string {
	var sb strings.Builder

	if s.Protocol == HTTP {
		sb.WriteString(fmt.Sprintf("%s %s", s.HTTPMethod, s.HTTPPath))
		return sb.String()
	}

	if s.Protocol == TCP {
		if s.Active {
			sb.WriteString(fmt.Sprintf(" Session Active"))
		} else {
			d := s.EndTime.Sub(s.StartTime)
			duration := durafmt.Parse(d)

			// fmt.Printf("%v - %v =  %v / %v\n", s.EndTime, s.StartTime, d, duration)
			sb.WriteString(fmt.Sprintf(" Session Life: %s", duration.Format(duraFormatOveride)))
		}
		return sb.String()
	}

	sb.WriteString(fmt.Sprintf("session type: %s", s.Protocol))
	return sb.String()
}

// SizeResult struct to store the size of a file in raw int64 and formatted values. Primarily for web template
type SizeResult struct {
	Val          int64
	FormattedVal string
}

// Size returns the SizeResult struct to return the raw size and formatted size to the template engine
func (s *Session) Size() *SizeResult {
	result := &SizeResult{}
	if s.Protocol == HTTP {
		result.Val = s.HTTPSession.ContentLength
		result.FormattedVal = humanize.Bytes(uint64(s.HTTPSession.ContentLength))
		return result
	}

	fi, err := os.Stat(s.SaveFile)
	if err != nil {
		result.Val = 0
		result.FormattedVal = fmt.Sprintf(" Unable to get file size: %v", err)
		return result
	}

	result.Val = fi.Size()
	result.FormattedVal = humanize.Bytes(uint64(fi.Size()))
	return result
}

// Bytes returns the bytes of the json formatted of the Session
func (s *Session) Bytes() []byte {
	dump, _ := json.MarshalIndent(s, "", "    ")
	return dump
}

// String returns the string of the json formatted of the Session
func (s *Session) String() string {
	dump, _ := json.MarshalIndent(s, "", "    ")
	return string(dump)
}


// IsValid returns state of the session, if the assets of the Session do not exist, will return false
func (s *Session) IsValid() (bool, error) {
	valid := FileExists(s.SaveFile)
	if !valid {
		return false, fmt.Errorf("%s does not exist", s.SaveFile)
	}

	for _, f := range s.MultiPartFiles {
		valid = FileExists(f.File)
		if !valid {
			return false, fmt.Errorf("%s does not exist", f.File)
		}
	}
	return true, nil
}

// InitializeHTTP update the Session with Http Request details
func (s *Session) InitializeHTTP(req *http.Request) {
	s.Protocol = HTTP
	_ = req.ParseForm()
	_ = req.ParseMultipartForm(MaxMultipartMemory)

	if req.MultipartForm != nil {
		for _, multiPartFileInfo := range req.MultipartForm.File {
			for _, fileInfo := range multiPartFileInfo {
				f, err := fileInfo.Open()
				if err != nil {
					fmt.Printf("%s error opening file: %v\n", fileInfo.Filename, err)
					continue
				}

				err = copyMultiPartFile(s, fileInfo, f)
				if err != nil {
					fmt.Printf("%s error writing file: %v\n", fileInfo.Filename, err)
					continue
				}

			}
		}
	}

	s.HTTPMethod = req.Method
	s.HTTPPath = req.RequestURI
	s.Active = true

	request := NewHTTPRequestJSON(req)
	if request != nil {
		s.HTTPSession = request
		dump, _ := json.MarshalIndent(request, "", "    ")
		_, _ = s.outputFile.Write(dump)
	}

	Broadcast(SessionUpdated, s.ToApiSession())
}

// LoadHTTPRequestJSON load request data file from disk if it is a http based session.
func (s *Session) LoadHTTPRequestJSON() error {
	if s.Protocol != HTTP {
		return fmt.Errorf("not a http protocol based session")
	}

	httpSessionBytes, err := ioutil.ReadFile(s.SaveFile)
	if err != nil {
		return err
	}

	request := &HTTPRequestJSON{}
	err = json.Unmarshal(httpSessionBytes, request)
	if err != nil {
		return err
	}

	s.HTTPSession = request
	return nil
}

// FileExists returns is a file exists on the local fs
func FileExists(name string) bool {
	fi, err := os.Stat(name)
	if err != nil {
		return false
	}

	if fi == nil {
		return false
	}
	return true
}

func copyMultiPartFile(session *Session, fileInfo *multipart.FileHeader, f multipart.File) error {

	sessionSaveDir := fmt.Sprintf("%s/%s", *saveDir, session.StartTime.Format("20060102"))
	sessionFileDir := fmt.Sprintf("%s/%s.files", sessionSaveDir, session.Key)
	_ = os.MkdirAll(sessionFileDir, 0777)

	file := fmt.Sprintf("%s/%s", sessionFileDir, fileInfo.Filename)

	destination, err := os.Create(file)
	if err != nil {
		fmt.Printf("Error Saving File: %s  - error: %v\n", file, err)
		return err
	}
	defer func() {
		_ = destination.Close()
	}()

	var nBytes int64
	nBytes, err = io.Copy(destination, f)
	if err != nil {
		return err
	}

	mpFile := &MultiPartFile{File: file, Size: nBytes, HumanSize: ByteCountDecimal(nBytes)}
	session.MultiPartFiles[fileInfo.Filename] = mpFile
	fmt.Printf("Saved File: %s  - bytes: %d\n", file, nBytes)
	return nil
}

func deactivateSession(session *Session) {
	if !session.Active {
		fmt.Printf("Skipping session %s - already saved\n", session.Key)
		return
	}
	fmt.Printf("Closing down session %s - %s\n", session.Key, session.SaveFile)

	_ = session.outputFile.Close()

	_ = m.BroadcastMultiple([]byte("session shut down"), session.Viewers)

	for _, v := range session.Viewers {
		_ = v.Close()
	}
	session.Active = false
	session.EndTime = time.Now()
	Broadcast(SessionUpdated, session.ToApiSession())
	_ = StoreSession(session)
}

// SaveAllSessions saves all open sessions to db. Called before shutdown
func SaveAllSessions() {
	for _, sess := range Sessions {
		if sess.Active {
			fmt.Printf("Saving session session %s\n", sess.Key)
			deactivateSession(sess)
		}
	}
}

func createSession(ip string) (*Session, error) {

	hashKey := make([]int64, 1)
	hashKey[0] = time.Now().Unix()
	key, _ := hasher.EncodeInt64(hashKey)

	session := &Session{}
	session.Key = key
	session.Viewers = make([]*melody.Session, 0)
	session.StartTime = time.Now()
	session.IP = ip
	session.Protocol = TCP
	session.MultiPartFiles = make(map[string]*MultiPartFile)
	sessionSaveDir := fmt.Sprintf("%s/%s", *saveDir, session.StartTime.Format("20060102"))
	sessionSaveFile := fmt.Sprintf("%s/%s.raw", sessionSaveDir, key)
	session.Active = true
	_ = os.MkdirAll(sessionSaveDir, 0777)
	outputFile, err := os.Create(sessionSaveFile)
	if err != nil {
		return nil, err
	}

	session.SaveFile = sessionSaveFile
	Sessions[key] = session
	session.outputFile = outputFile
	err = StoreSession(session)

	Broadcast(SessionCreated, session.ToApiSession())

	return session, err
}

func createNewSessionResponse(session *Session) gin.H {

	viewURL := fmt.Sprintf("http://%s:%d/v/%s", *publicIP, *publicHttpPort, session.Key)
	textURL := fmt.Sprintf("http://%s:%d/t/%s", *publicIP, *publicHttpPort, session.Key)
	infoURL := fmt.Sprintf("http://%s:%d/api/info/%s", *publicIP, *publicHttpPort, session.Key)

	pay := gin.H{
		"code":    "SESSION_CREATED",
		"name":    session.Key,
		"viewURL": viewURL,
		"textURL": textURL,
		"infoURL": infoURL,
	}

	if session.Protocol == HTTP {
		pay["requestBodyURL"] = fmt.Sprintf("http://%s:%d/t/%s/body", *publicIP, *publicHttpPort, session.Key)

		for k := range session.MultiPartFiles {
			pay[fmt.Sprintf("uploadedFile_%s_URL", k)] = fmt.Sprintf("http://%s:%d/t/%s/%s", *publicIP, *publicHttpPort, session.Key, k)
		}
	}

	return pay
}

// PurgeSession removes a session and removes all assets and references in db
func PurgeSession(s *Session) {

	fmt.Printf("Purging Session: %v\n", s.Key)

	_ = os.Remove(s.SaveFile)
	for _, f := range s.MultiPartFiles {
		_ = os.Remove(f.File)
	}

	err := DeleteSession(s.Key)
	if err != nil {
		fmt.Printf("Error deleting session: %s error: %v\n", s.Key, err)
	}

	delete(Sessions, s.Key)
	Broadcast(SessionDeleted, s.Key)
}
