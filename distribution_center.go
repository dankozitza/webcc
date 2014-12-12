package main

import (
	"fmt"
	"github.com/dankozitza/dkutils"
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

var log_file string = "dc.log"
var access_log string = "dc_access.log"

var conf sconf.Sconf = sconf.Init("config.json",
	sconf.Sconf{"logtrack_default_log_file": log_file})

var client_config_file = "client_config.json"
var client_conf sconf.Sconf = sconf.New(client_config_file,
	sconf.Sconf{
		"Links": map[string]interface{}{
			"client conf":         "/clientconf",
			"statdist":            "/statdist",
			"stdout":              "/stdout",
			"access":              "/access",
			"distribution center": "/dc",
			"file server":         "http://localhost:9001"}})

var log logtrack.LogTrack = logtrack.New()
var access logtrack.LogTrack = logtrack.New()

var stat stattrack.StatTrack = stattrack.New("test distribution center")

type staticfile string

func (f staticfile) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	access.P(r.RemoteAddr, " ", r, "\n")

	err := client_conf.Update(client_config_file)
	if err != nil {
		stat.Warn("could not update client_conf: " + err.Error())

	} else {
		stat.Pass("")
	}
	fix_config_links()

	fi, err := os.Open(string(f))
	if err != nil {
		stat.Warn("failed to open file: " + string(f))
		return
	}

	buff := make([]byte, 1024)
	for {
		n, err := fi.Read(buff)
		if err != nil && err != io.EOF {
			stat.PanicErr("error while reading "+string(f), err)
		}
		if n == 0 {
			break
		}

		fmt.Fprint(w, string(buff[:n]))
	}
	fi.Close()
	//stat.Pass("served " + fmt.Sprint(r.URL) + " to " + r.RemoteAddr)
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

	// typecheck client_conf and set defaults
	fix_config_links()
	var links map[string]interface{}
	links = client_conf["Links"].(map[string]interface{})

	access.Log_file = access_log
	access.To_Stdout = false

	var fsh myFileServer
	s := &http.Server{
		Addr:           "localhost:9001",
		Handler:        fsh,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 22,
	}
	go s.ListenAndServe()

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

	log.P("starting http server\n")

	log.P(http.ListenAndServe("localhost:9000", nil))
	return
}

func fix_config_links() {

	var links map[string]interface{}

	cpy := client_conf["Links"]
	err := dkutils.ForceType(&cpy, links)
	if err != nil {
		stat.Warn(err.Error())
	}
	client_conf["Links"] = cpy

	// if the type of client_conf["Links"] is wrong
	switch v := client_conf["Links"].(type) {

	case map[string]interface{}:
		links = v

	// otherwise warn and overwrite it
	default:
		stat.Warn("client_config[\"Links\"] is not type map[string]interface{}." +
			" Making it type map[string]interface{}. Check sconf config file: " +
			client_config_file)
		var freshlinks map[string]interface{}
		client_conf["Links"] = freshlinks
		links = client_conf["Links"].(map[string]interface{})
	}

	// set defaults in links
	for k, _ := range links {

		// if the type of links[k] is correct
		switch v := links[k].(type) {

		// check that it is set
		case string:
			// set default
			if v == "" {
				links[k] = "/" + k
			}

		// otherwise warn and set default
		default:
			stat.Warn("Links[\"" + k + "\"] is not type string. Check sconf " +
				"config file: " + client_config_file + ". Setting Links[\"" + k +
				"\"] to default: /" + k)
			links[k] = "/" + k
			return
		}
	}
}