package main

import (
	"fmt"
	"github.com/dankozitza/logdist"
	"github.com/dankozitza/logtrack"
	"github.com/dankozitza/sconf"
	"github.com/dankozitza/statdist"
	"github.com/dankozitza/stattrack"
	"io"
	"net/http"
	"os"
	"time"
)

var log_file string = "cc.log"
var access_log string = "cc_access.log"

var conf sconf.Sconf = sconf.Init("config.json",
	sconf.Sconf{"logtrack_default_log_file": log_file})

var log logtrack.LogTrack = logtrack.New()
var access logtrack.LogTrack = logtrack.New()

var stat stattrack.StatTrack = stattrack.New("test control center")

type staticfile string

func (f staticfile) ServeHTTP(w http.ResponseWriter, r *http.Request) {

   access.P(r.RemoteAddr, " ", r, "\n")

	fi, err := os.Open(string(f))
	if err != nil {
		stat.Warn("failed to open file: " + string(f))
		return
	}

	buff := make([]byte, 1024)
	for {
		n, err := fi.Read(buff)
		if err != nil && err != io.EOF {
			panic(err)
		}
		if n == 0 {
			break
		}

		fmt.Fprint(w, string(buff[:n]))
	}
	fi.Close()
	stat.Pass("served " + fmt.Sprint(r.URL) + " to " + r.RemoteAddr)
}

var fsh http.Handler = http.FileServer(http.Dir("/tmp/static"))

type myFileServer struct{}

func (mfs myFileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	access.P(r.RemoteAddr, " ", r, "\n")

   fsh.ServeHTTP(w, r)
	//access.P("---response: ", w, "\n")

   stat.Pass("served " + fmt.Sprint(r.URL) + " to " + r.RemoteAddr)
}

func main() {

	access.Set_log_file_path(access_log)
   access.To_Stdout = false

	var fsh myFileServer // = http.FileServer(http.Dir("/tmp/static"))
	s := &http.Server{
		Addr:           "localhost:8999",
		Handler:        fsh,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 22,
	}
	go s.ListenAndServe()

	client_conf := sconf.New("client_config.json", nil)
   links := client_conf["Links"].(map[string]interface{})
   log.P(links, "\n")
	var cli sconf.HTTPHandler = sconf.HTTPHandler(client_conf)
	http.Handle(links["client conf"].(string), cli)

	var jsm statdist.HTTPHandler
	http.Handle(links["statdist"].(string), jsm)

	var sldh logdist.HTTPHandler = "stdout"
	http.Handle(links["stdout"].(string), sldh)

	var ldh logdist.HTTPHandler = logdist.HTTPHandler(log_file)
	http.Handle("/logfile", ldh)

	var ah logdist.HTTPHandler = logdist.HTTPHandler(access_log)
	http.Handle(links["access"].(string), ah)

	var in staticfile = "index.htm"
	http.Handle(links["distribution center"].(string), in)

	log.P(1 << 22, "\n")

	log.P("starting http server\n")

	log.P(http.ListenAndServe("localhost:9000", nil))
	return
}
