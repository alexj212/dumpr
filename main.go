// Copyright 2021 Alex jeannopoulos. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"fmt"
	"github.com/droundy/goopt"
	"github.com/hako/durafmt"
	"github.com/potakhov/loge"
	"github.com/speps/go-hashids/v2"
	"gopkg.in/olahol/melody.v1"
	"io/fs"
	"net/http"
	"os"
	"time"
)

var (

	// MaxMultipartMemory max multipart upload size
	MaxMultipartMemory int64 = 8 << 20 // 8 MiB

	// BuildDate date string of when build was performed filled in by -X compile flag
	BuildDate string

	// GitRepo string of the git repo url when build was performed filled in by -X compile flag
	GitRepo string

	// BuiltBy date string of who performed build filled in by -X compile flag
	BuiltBy string

	// CommitDate date string of when commit of the build was performed filled in by -X compile flag
	CommitDate string

	// Branch string of branch in the git repo filled in by -X compile flag
	Branch string

	// LatestCommit date string of when build was performed filled in by -X compile flag
	LatestCommit string

	// Version string of build filled in by -X compile flag
	Version string

	// OsSignal signal used to shut down
	OsSignal chan os.Signal

	saveDir = goopt.String([]string{"--saveDir"}, "/tmp", "save directory")
	webDir  = goopt.String([]string{"--webDir"}, "./web", "web assets directory")
	//quiet              = goopt.Flag([]string{"--quiet"}, []string{}, "silently log to file", "")
	serverHost        = goopt.String([]string{"--host"}, "0.0.0.0", "host for server")
	publicUrl         = goopt.String([]string{"--publicUrl"}, "http://127.0.0.1:8080", "public url")
	publicBinEndpoint = goopt.String([]string{"--publicBinEndpoint"}, "127.0.0.1:8081", "public url")

	httpPort = goopt.Int([]string{"--port"}, 8080, "port for server")
	tcpPort  = goopt.Int([]string{"--tcpport"}, 8081, "tcp port for server")

	exportTemplates   = goopt.Flag([]string{"--export"}, nil, "export templates to --webDir value.", "")
	purgeOlderThanStr = goopt.String([]string{"--purgeOlderThan"}, "24h", "Purge sessions from disk older than value. 0 will disable.")
	maxSessionSz      = goopt.Int([]string{"--maxSessionSize"}, 1, "maximum session size in mb.")
	hasher            *hashids.HashID
	webFS             fs.FS
	webDirHTTPFS      http.FileSystem
	staticDirHTTPFS   http.FileSystem

	m                       melody.Melody
	duraFormatOveride       durafmt.Units
	purgeOlderThan          *durafmt.Durafmt
	maxSessionSize          int
	maxSessionSizeFormatted string
)

func init() {
	// Setup goopts
	goopt.Description = func() string {
		return "Http and TCP logger endpoint"
	}
	goopt.Author = "Alex Jeannopoulos"
	goopt.ExtraUsage = ``
	goopt.Summary = `
dumpr
        dumpr will create and http and tcp listener and log connections and inbound traffic to a log file.

`

	goopt.Version = fmt.Sprintf(
		`dumpr! build information

  Version         : %s
  Git repo        : %s
  Git commit      : %s
  Git branch      : %s
  Commit date     : %s
  Build date      : %s
  Built By        : %s
`, Version, GitRepo, LatestCommit, Branch, CommitDate, BuildDate, BuiltBy)

	//Parse options
	goopt.Parse(nil)

	var err error
	duraFormatOveride, err = durafmt.DefaultUnitsCoder.Decode("y:y,w:w,d:d,h:h,m:m,s:s,ms:ms,μs:μs")
	if err != nil {
		panic(err)
	}
}

func main() {
	var err error
	purgeOlderThan, err = durafmt.ParseString(*purgeOlderThanStr)
	if err != nil {
		fmt.Printf("Invalid field: purgeInterval - %v\n", err)
		os.Exit(1)
	}

	maxSessionSize = *maxSessionSz << (10 * 2) // 2 refers to the constants ByteSize MB

	maxSessionSizeFormatted = ByteCountDecimal(int64(maxSessionSize))

	OsSignal = make(chan os.Signal, 1)

	hd := hashids.NewData()
	hd.Salt = "dumpr salt"
	hd.MinLength = 30
	hasher, _ = hashids.NewWithData(hd)

	logeShutdown := loge.Init(
		loge.Path("."),
		loge.EnableOutputConsole(true),
		loge.EnableOutputFile(false),
		loge.ConsoleOutput(os.Stdout),
		loge.EnableDebug(),
		loge.EnableError(),
		loge.EnableInfo(),
		loge.EnableWarning(),
		//loge.Transports(func(list loge.TransactionList) []loge.Transport {
		//    // transport := loge.WrapTransport(list, c)
		//    return []loge.Transport{transport}
		//}),
	)

	defer logeShutdown()

	webFS, err = SetupFS()
	if err != nil {
		fmt.Printf("Error initializing assets fs, error: %v\n", err)
		return
	}

	webDirHTTPFS = http.FS(webFS)
	staticDirHTTPFS = EmbedFolder(webFS, "assets", false)

	if *exportTemplates {
		fmt.Printf("Exporting templates to %s\n", *webDir)
		err = copyTemplatesToTarget(*webDir)
		if err != nil {
			fmt.Printf("Error saving templates, error: %v\n", err)
		}
		return
	}

	db, err = InitializeDB()
	if err != nil {
		fmt.Printf("Error initializing db, error: %v\n", err)
		return
	}
	defer func() {
		_ = db.Close()
	}()

	sessionList, err := LoadSessions()
	if err != nil {
		fmt.Printf("Error loading sessions, error: %v\n", err)
		return
	}
	fmt.Printf("Loaded from db %d sessions\n", len(sessionList))

	if purgeOlderThan.Duration() > 0 {
		go LaunchSessionReaper()
	}

	go LaunchSessionUpdater()

	err = InitializeAutoResponders()
	if err != nil {
		fmt.Printf("Error loading auto responders, error: %v\n", err)
		return
	}

	err = GinServer()
	if err != nil {
		fmt.Printf("Error launching web endpoint, error: %v\n", err)
		return
	}

	err = SpawnTCPListener(*serverHost, *tcpPort)
	if err != nil {
		fmt.Printf("Error launching tcp endpoint, error: %v\n", err)
		return
	}

	LoopForever(func() {
		fmt.Printf("Saving all sessions\n")
		SaveAllSessions()
		fmt.Printf("Saved all sessions\n")
	})
}

// LaunchSessionReaper launches the session reaper that will cleanup sessions older than purgeOlderThan option.
func LaunchSessionReaper() {
	fmt.Printf("launching cleanup process, will delete sessions older than %v\n", purgeOlderThan)

	for {
		fmt.Printf("Running session cleanup: %v+\n", time.Now().Format(time.ANSIC))

		purgeSessionList := make([]*Session, 0)
		for _, v := range Sessions {
			purgeTime := v.StartTime.Add(purgeOlderThan.Duration())

			//fmt.Printf("Session Start Time%v : %v\n", v.StartTime, purgeTime)

			if time.Now().After(purgeTime) && !v.Active {
				//fmt.Printf("File older than %v : %v\n", purgeOlderThan, purgeTime)
				purgeSessionList = append(purgeSessionList, v)
			}
		}

		for _, v := range purgeSessionList {
			//fmt.Printf("[%d] purging session: %s\n", i, v.Key)
			PurgeSession(v)
		}

		time.Sleep(time.Minute * 1)
	}
}

// LaunchSessionUpdater launches the session updater that will send updates for active sessions.
func LaunchSessionUpdater() {
	fmt.Printf("launching session updater process, will update sessions every 10 seconds\n")

	for {
		for _, v := range Sessions {
			if v.Active {
				Broadcast(SessionUpdated, v.ToApiSession())
			}
		}

		time.Sleep(time.Second * 10)
	}
}
