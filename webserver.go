// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"fmt"
	"github.com/foolin/goview"
	"github.com/foolin/goview/supports/ginview"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/render"
	"gopkg.in/olahol/melody.v1"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

func createDefaultPageData(pageName string, session *Session) gin.H {
	data := gin.H{
		"title":                   pageName,
		"publicIP":                *publicIP,
		"publicHttpPort":          *publicHttpPort,
		"publicTCPPort":           *publicTCPPort,
		"httpPort":                *httpPort,
		"purgeOlderThan":          purgeOlderThan.String(),
		"maxSessionSize":          maxSessionSize,
		"maxSessionSizeFormatted": maxSessionSizeFormatted,
		"autoResponderCount":      autoResponders.Size(),
		"Sessions":                GetAllSessions(),
		"ActiveSessions":          GetActiveSessions(),
		"InActiveSessions":        GetInActiveSessions(),
		"add": func(a int, b int) int {
			return a + b
		},
	}

	if session != nil {
		data["session"] = session
	}
	return data

}

// WriteFunc convert func to io.Writer.
type WriteFunc func([]byte) (int, error)

// Write, will write bytes to the WriteFunc
func (fn WriteFunc) Write(data []byte) (int, error) {
	return fn(data)
}

// NewCustomWriter return a new io.Writer
func NewCustomWriter() io.Writer {
	return WriteFunc(func(data []byte) (int, error) {
		fmt.Printf(">> %s\n", string(data))
		return 0, nil
	})
}

// GinServer launch gin server
func GinServer() (err error) {
	//gin.DefaultWriter= NewCustomWriter()
	//gin.DefaultErrorWriter= NewCustomWriter()
	gin.SetMode(gin.ReleaseMode)
	//gin.DebugPrintRouteFunc = func(httpMethod, absolutePath, handlerName string, nuHandlers int) {
	//    fmt.Printf("httpMethod: %s %s %s %d\n", httpMethod, absolutePath, handlerName, nuHandlers)
	//}

	router := gin.Default()

	router.MaxMultipartMemory = MaxMultipartMemory

	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	router.Use(cors.New(config))

	// templateConfig engine config
	templateConfig := goview.Config{
		Root:         "",
		Extension:    ".html",
		Master:       "layouts/master",
		Partials:     []string{"partials/howto"},
		Funcs:        make(template.FuncMap),
		DisableCache: true,
		Delims:       goview.Delims{Left: "{{", Right: "}}"},
	}

	templateConfig.Funcs["FileSize"] = func(fileName string) string {
		fi, err := os.Stat(fileName)
		if err != nil {
			return err.Error()
		}
		// get the size
		size := fi.Size()

		return ByteCountDecimal(size)
	}

	templateConfig.Funcs["ADD"] = func(a int, b int) int {
		return a + b
	}

	//new template engine
	templateEngine := ginview.New(templateConfig)
	templateEngine.SetFileHandler(func(config goview.Config, tplFileBaseName string) (content string, err error) {

		tplFileName := fmt.Sprintf("%s%s", tplFileBaseName, config.Extension)

		// fmt.Printf("templateEngine load: %s\n", tplFileName)
		tplFile, err := webFS.Open(tplFileName)
		if err != nil {
			return "", fmt.Errorf("ViewEngine tplFileName:%v error: %v", tplFileName, err)
		}

		data, err := ioutil.ReadAll(tplFile)
		if err != nil {
			return "", fmt.Errorf("ViewEngine render read name:%v, error: %v", tplFileName, err)
		}
		return string(data), nil
	})
	router.HTMLRender = templateEngine

	m := melody.New()
	m.Upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	router.GET("/favicon.ico", func(ctx *gin.Context) {
		ctx.FileFromFS("favicon.ico", webDirHTTPFS)
	})

	router.GET("/dumpr.png", func(ctx *gin.Context) {
		ctx.FileFromFS("dumpr.png", webDirHTTPFS)
	})

	router.StaticFS("/assets/", staticDirHTTPFS)

	router.GET("/", func(ctx *gin.Context) {
		data := createDefaultPageData("Session List", nil)
		//render with master
		ctx.HTML(http.StatusOK, "index", data)
	})

	router.GET("/api/list/active", func(ctx *gin.Context) {
		ctx.JSON(200, GetActiveSessions())
	})
	router.GET("/api/list/inactive", func(ctx *gin.Context) {
		ctx.JSON(200, GetInActiveSessions())
	})

	router.GET("/api/list/sessions", func(ctx *gin.Context) {
		ctx.JSON(200, GetAllSessions())
	})

	router.GET("/api/info/:name", func(ctx *gin.Context) {

		name := ctx.Param("name")
		sess, _ := Sessions[name]

		if sess != nil {
			ctx.JSON(200, sess)
		} else {
			ctx.JSON(404, gin.H{"code": "SESSION_NOT_FOUND", "message": "Session not found"})
		}
	})

	router.GET("/api/autoresponder/:name", func(ctx *gin.Context) {

		name := ctx.Param("name")

		responder, ok := autoResponders.Get(name)
		if !ok {
			payload := gin.H{
				"code":    "AutoResponder-NOT-FOUND",
				"message": fmt.Sprintf("autoresponder [%s] not found", name),
			}

			ctx.JSON(404, payload)
			return
		}

		ctx.JSON(200, responder)
	})
	router.DELETE("/api/autoresponder/:name", func(ctx *gin.Context) {
		name := ctx.Param("name")

		_, ok := autoResponders.Get(name)
		if !ok {
			payload := gin.H{
				"result":  "failed",
				"code":    "AutoResponder-NOT-FOUND",
				"message": fmt.Sprintf("autoresponder [%s] not found", name),
			}
			ctx.JSON(404, payload)
			return
		}

		success := autoResponders.Delete(name)
		if !success {
			payload := gin.H{
				"result":  "failed",
				"code":    "AutoResponder-NOT-FOUND",
				"message": fmt.Sprintf("DeleteAutoResponder [%s] returned false", name),
			}
			ctx.JSON(404, payload)
			return
		}

		payload := gin.H{
			"result":  "success",
			"code":    "SUCCESS",
			"message": fmt.Sprintf("autoresponder [%s] deleted", name)}
		ctx.JSON(200, payload)
	})

	router.POST("/api/autoresponder/new", func(ctx *gin.Context) {
		var payload AutoResponse

		// Call BindJSON to bind the received JSON to AutoResponse.
		if err := ctx.BindJSON(&payload); err != nil {
			response := gin.H{
				"result":  "failed",
				"code":    "AutoResponder-NOT-FOUND",
				"message": fmt.Sprintf("unable to parse json"),
			}

			ctx.JSON(404, response)
			return
		}

		err = autoResponders.Insert(&payload)
		if err != nil {
			response := gin.H{
				"result":  "failed",
				"code":    "AutoResponder-NOT-FOUND",
				"message": fmt.Sprintf("autoresponder [%s] not inserted", payload.Name),
				"error":   err.Error(),
			}
			ctx.JSON(404, response)
			return
		}

		response := gin.H{
			"result":  "success",
			"code":    "SUCCESS",
			"message": fmt.Sprintf("autoresponder [%s] inserted", payload.Name),
		}
		ctx.JSON(200, response)
	})

	router.PUT("/api/autoresponder/:name", func(ctx *gin.Context) {
		var err error
		name := ctx.Param("name")

		fmt.Printf("autoresponder[%s]\n", name)
		var payload AutoResponse
		// Call BindJSON to bind the received JSON to AutoResponse.
		if err = ctx.BindJSON(&payload); err != nil {
			response := gin.H{
				"result":  "failed",
				"code":    "AutoResponder-NOT-FOUND",
				"message": fmt.Sprintf("unable to parse json [%s]", name),
				"err":     fmt.Errorf("%v", err),
			}
			ctx.JSON(404, response)
			return
		}

		// fmt.Printf("autoresponder[%s]\n", name)
		_, ok := autoResponders.Get(name)
		if ok {
			err = autoResponders.Update(&payload)
			if err != nil {
				result := gin.H{
					"result":  "failed",
					"code":    "AutoResponder-NOT-FOUND",
					"message": fmt.Sprintf("unable to update autoresponder [%s]", name),
					"error":   err.Error(),
				}

				ctx.JSON(404, result)
				return
			}

			fmt.Printf("autoresponder[%s] %s\n", name, payload.String())

			result := gin.H{
				"result":        "success",
				"code":          "SUCCESS",
				"message":       fmt.Sprintf("autoresponder [%s] updated", name),
				"autoresponder": payload}

			ctx.JSON(200, result)
			return
		}

		err = autoResponders.Insert(&payload)
		result := gin.H{
			"result":        "success",
			"code":          "SUCCESS",
			"message":       fmt.Sprintf("autoresponder %s added", name),
			"autoresponder": payload,
		}

		if err != nil {
			result["error"] = err
		}
		ctx.JSON(200, result)
	})

	router.GET("/api/autoresponder/list", func(ctx *gin.Context) {
		ctx.JSON(200, autoResponders.l)
	})

	router.GET("/v/:name", func(c *gin.Context) {
		name := c.Param("name")

		session, ok := Sessions[name]

		if !ok {
			c.String(http.StatusNotFound, "session not found")
			return
		}
		data := createDefaultPageData("session details!", session)

		if session.Protocol == HTTP {
			c.HTML(http.StatusOK, "http_view", data)
		} else {
			data["sse_url"] = "./ws"
			c.HTML(http.StatusOK, "live_view", data)
		}
	})

	router.GET("/t/:name/:filename", func(c *gin.Context) {
		name := c.Param("name")
		filename := c.Param("filename")
		session, ok := Sessions[name]
		if !ok {
			c.String(http.StatusNotFound, "session not found")
			return
		}

		fileInfo, ok := session.MultiPartFiles[filename]
		if !ok {
			c.String(http.StatusNotFound, "file not found")
			return
		}
		c.File(fileInfo.File)
	})

	router.GET("/t/:name", func(c *gin.Context) {
		name := c.Param("name")
		session, ok := Sessions[name]
		if !ok {
			c.String(http.StatusNotFound, "session not found")
			return
		}

		if session.Protocol == HTTP {
			c.Header("Content-Type", "application/json; charset=utf-8")
			c.Header("Cache-Control", "no-cache")
			c.File(session.SaveFile)
		} else {
			c.File(session.SaveFile)
		}
	})

	router.GET("/t/:name/body", func(c *gin.Context) {
		name := c.Param("name")
		session, ok := Sessions[name]
		if !ok {
			c.String(http.StatusNotFound, "session not found")
			return
		}

		if session.Protocol == HTTP && session.HTTPSession != nil && session.HTTPSession.Body != nil {
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "application/json; charset=utf-8", session.HTTPSession.Body)
		} else {
			c.String(http.StatusNotFound, "http session not found")
			return
		}
	})

	router.GET("/v/:name/ws", func(c *gin.Context) {
		name := c.Param("name")
		session, ok := Sessions[name]
		if !ok {
			c.String(http.StatusNotFound, "session not found")
			return
		}

		keys := map[string]interface{}{
			"name":    name,
			"session": session,
		}
		_ = m.HandleRequestWithKeys(c.Writer, c.Request, keys)
	})

	router.GET("/about", func(ctx *gin.Context) {
		data := createDefaultPageData("About dumpr!", nil)
		//render with master
		ctx.HTML(http.StatusOK, "about", data)
	})

	router.GET("/responders", func(ctx *gin.Context) {
		data := createDefaultPageData("dumpr! autoresponders", nil)
		//render with master
		ctx.HTML(http.StatusOK, "responders", data)
	})
	SetupSSERouter(router, "/stream")

	router.NoRoute(func(c *gin.Context) {
		session, err := createSession(c.ClientIP())

		if err != nil {
			c.JSON(500, gin.H{"code": "CREATE_SESSION_FAILED",
				"message": err.Error(),
			})
			return
		}

		session.InitializeHTTP(c.Request)

		c.Header("X-Session-Key", session.Key)
		url := fmt.Sprintf("http://%s:%d/t/%s", *publicIP, *publicHttpPort, session.Key)
		c.Header("X-Session-URL", url)
		url = fmt.Sprintf("http://%s:%d/api/info/%s", *publicIP, *publicHttpPort, session.Key)
		c.Header("X-Session-Info-URL", url)

		autoResponse := autoResponders.Find(c.Request)
		if autoResponse != nil {
			session.HandledByRule = autoResponse.Name
		}
		deactivateSession(session)

		if autoResponse != nil {
			c.Header("X-AutoResponder-Name", autoResponse.Name)
			if autoResponse.ResponseHeaders != nil {
				for k, v := range autoResponse.ResponseHeaders {
					c.Header(k, v)
				}
			}

			payload := []byte(autoResponse.Response)
			c.Data(autoResponse.StatusCode, autoResponse.ContentType, payload)
		} else {
			sessionInfo := createNewSessionResponse(session)
			c.Render(200, render.JSON{Data: sessionInfo})
		}

	})

	m.HandleConnect(func(s *melody.Session) {

		sessionObj, ok := s.Keys["session"]
		if !ok {
			_ = s.CloseWithMsg([]byte("session not found"))
			return
		}

		var session *Session
		session, ok = sessionObj.(*Session)
		if !ok {
			panic("Unable to cast s.Keys[\"session\"] to *Session ")
		}

		historyFile, err := os.Open(session.SaveFile)
		if err != nil {
			_ = s.CloseWithMsg([]byte("unable to read file"))
			return
		}
		defer func() {
			_ = historyFile.Close() //Do not forget to close the file
		}()

		bytesSent := 0
		r := bufio.NewReader(historyFile)
		for {
			buf := make([]byte, 4*1024) //the chunk size
			n, err := r.Read(buf)       //loading chunk into buffer
			buf = buf[:n]
			if n == 0 {
				if err != nil {
					break
				}
				if err == io.EOF {
					break
				}
			}
			bytesSent += len(buf)
			_ = s.Write(buf)
		}
		//fmt.Printf("HandleConnect: name: %s bytesSent: %d\n", name, bytesSent)
		session.Viewers = append(session.Viewers, s)
	})

	m.HandleDisconnect(func(s *melody.Session) {

		sessionObj, ok := s.Keys["session"]
		if !ok {
			_ = s.CloseWithMsg([]byte("session not found"))
			return
		}

		var session *Session
		session, ok = sessionObj.(*Session)
		if !ok {
			panic("Unable to cast s.Keys[\"session\"] to *Session ")
		}

		session.Viewers = removeElement(session.Viewers, s)
		//fmt.Printf("HandleDisconnect: name: %s viewer cnd: %d file: %s\n", name, len(session.Viewers), session.SaveFile)
	})

	m.HandleMessage(func(s *melody.Session, msg []byte) {
		_ = m.BroadcastFilter(msg, func(q *melody.Session) bool {
			return q.Request.URL.Path == s.Request.URL.Path
		})
	})

	go func() {
		err = router.Run(fmt.Sprintf("%s:%d", *serverHost, *httpPort))
		if err != nil {
			log.Fatalf("Error starting server, the error is '%v'", err)
		}
	}()

	return
}

func removeElement(s []*melody.Session, session *melody.Session) []*melody.Session {
	index := linearSearch(s, session)
	if index != -1 {
		return append(s[:index], s[index+1:]...)
	}
	return s
}

func linearSearch(s []*melody.Session, session *melody.Session) int {
	for i, n := range s {
		if n == session {
			return i
		}
	}
	return -1
}
