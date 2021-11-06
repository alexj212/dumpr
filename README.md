## dumpr
![dumpr](./web/favicon-32x32.png "dumpr")

A tcp / http request dumpr endpoints. The project was inspired by seashells.io, pastebin and other projects. The server will expose 2 ports, a http server and a tcp server. All connection info along with inbound traffic is saved to disk. 
While a connection is active, a url is available that will provide live updates to the session log file. A session log can be downloaded as well. If http traffic is detected the session is decoded and information is saved to disk. The data is in a json format, will list path, protocol, headers and body. Multipart form uploads are parsed and saved to disk as well.
Various web service urls are available to list sessions, pull session info and files. 

## Auto Responder
Another ability that dumpr has is that it can be setup to return a custom response based on request pattern match. A match is made against the 'method' and 'path'. Any match that is made will send a response with the 'status_code', 'content-type' and 'response'. The 'method' and 'path' values can be regular expressions.

The auto responder system uses a yaml file to define the rules for the auto responder. The yml file can be defined with 

```bash
  --responses=responses.yaml    auto responder file
```

### responses.yaml 
```.yaml
responses:
  - method: .*
    name: rule 1
    path: /hello\.(txt|text)
    status_code: 200
    content_type: text/plain
    response: |
      Hello World!!!!
  - method: .*
    name: rule 2
    path: /hello\.json
    status_code: 200
    content_type: text/json
    response: |
      {
        "message": "Hello World"
      }

```

### Rule 1: 
* Will match any http method and path with the regular expression `/hello\.(txt|text)` 
* Will respond with status code 200, content_type `text/plain` and the response body `Hello World!!!!` 

### Rule 2: 
* Will match any http method and path with the regular expression `/hello\.json` 
* Will respond with status code 200, content_type `text/json` and the response body `{ "message": "Hello World" }` 



## Web Service URLS
Web service urls are provided to access list of session, session info, assets and auto responder rules.   



```
/                           - html listing of sessions
/about                      - about the project
/t/:name                    - return the log file for a session.
/t/:name/:filename          - return a file uploaded in a multi part upload session.
/v/:name                    - live view html page    
/v/:name/ws                 - websocket for live updated for a session log file.
/api/list/sessions          - return a json array of all sessions.
/api/list/active            - return a json array of active sessions.
/api/list/inactive          - return a json array of inactive sessions.
/api/info/:name             - return json structure of the session.
/api/autoresponder/:id      - return auto responder for rule id.
/api/autoresponder/list     - return list of all auto responders.



Any unknown url is logged.
```


## WebCapture



```bash
$ http -f POST http://127.0.0.1:8081/hello.txt
POST /hello.txt HTTP/1.1
Accept: */*
Accept-Encoding: gzip, deflate
Connection: keep-alive
Content-Length: 0
Content-Type: application/x-www-form-urlencoded; charset=utf-8
Host: 127.0.0.1:8081
User-Agent: HTTPie/2.5.0



HTTP/1.1 200 OK
Content-Length: 16
Content-Type: text/plain
Date: Wed, 03 Nov 2021 00:23:55 GMT
X-AutoResponder-ID: 0
X-AutoResponder-Name: rule 1
X-Session-Info-URL: http://127.0.0.1:8080/api/info/92WQZePap1mwMGMDyVN0G7Ld4zlk3J
X-Session-Key: 92WQZePap1mwMGMDyVN0G7Ld4zlk3J
X-Session-URL: http://127.0.0.1:8080/v/92WQZePap1mwMGMDyVN0G7Ld4zlk3J

Hello World!!!!



```


## Http Headers
```
X-AutoResponder-ID: 0
X-AutoResponder-Name: rule 1


X-Session-Info-URL: http://127.0.0.1:8080/api/info/92WQZePap1mwMGMDyVN0G7Ld4zlk3J
X-Session-Key: 92WQZePap1mwMGMDyVN0G7Ld4zlk3J
X-Session-URL: http://127.0.0.1:8080/v/92WQZePap1mwMGMDyVN0G7Ld4zlk3J

```



## Building

```.bash
make dumpr
```
Will create a binary `./bin/dumpr`

## Running

```.bash
./bin/dumpr
```

This will launch the web server on localhost:8080 and the tcp server on 8081. A Session is stored in the following /tmp/<date>/<session id>



## Options

```.bash
Options:
  --saveDir=/tmp                save directory
  --webDir=./web                web assets directory
  --quiet                       silently log to file
  --host=0.0.0.0                host for server
  --publicUrl=http://127.0.0.1  public url
  --publicPort=8080             public port for http server
  --tcpport=8081                tcp port for server
  --port=8080                   port for server
  --responses=responses.yaml    auto responder file
  -h, --help                    Show usage message
  --version                     Show version
```









