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
)

var log_file string = "cc.log"

var conf sconf.Sconf = sconf.Init("config.json",
	sconf.Sconf{"logtrack_default_log_file": log_file})

var log logtrack.LogTrack = logtrack.New()
var access logtrack.LogTrack = logtrack.New()

var stat stattrack.StatTrack = stattrack.New("test control center")

type staticfile string

func (f staticfile) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	access.P(r, "\n")

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

func main() {

	access.Set_log_file_path("cc_access.log")

	var jsm statdist.HTTPHandler
	http.Handle("/statdist", jsm)

	var sldh logdist.HTTPHandler = "stdout"
	http.Handle("/stdout", sldh)

	var ldh logdist.HTTPHandler = logdist.HTTPHandler(log_file)
	http.Handle("/logfile", ldh)

	var in staticfile = "index.htm"
	http.Handle("/cc", in)

	client_conf := sconf.New("client_config.json", nil)
	var cli sconf.HTTPHandler = sconf.HTTPHandler(client_conf)
	http.Handle("/clientconf", cli)

	var fsh http.Handler = http.StripPrefix(
		"/fs/",
		http.FileServer(http.Dir("/tmp/static")))
	http.Handle("/fs/", fsh)

	log.P("starting http server\n")

	http.ListenAndServe("localhost:9000", nil)
	return
}
