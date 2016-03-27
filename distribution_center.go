package main

import (
	"flag"
	"fmt"
	"github.com/dankozitza/dkutils"
	"github.com/dankozitza/ffetcher"
	"github.com/dankozitza/logdist"
	"github.com/dankozitza/logtrack"
	"github.com/dankozitza/sconf"
	"github.com/dankozitza/statdist"
	"github.com/dankozitza/stattrack"
	"io"
	"net/http"
	"os"
)

var (
	address    = flag.String("h", "localhost", "ip address to host on")
	port       = flag.String("p", "9000", "port to host on")
	ftpport    = flag.String("f", "9001", "port to host ftp on")
	log_file	  = flag.String("l", "dc.log", "log file to print to")
	access_log = flag.String(
		"a", "dc_access.log", "log file to print http logs")
	ffetch_conf_file = flag.String(
		"fc", "ffetch_config.json", "json config file for ffetch")
	client_conf_file = flag.String(
		"cc", "client_config.json", "json config file for client")
	conf_file  = flag.String("c", "config.json", "json config file for server")
)

var (
	conf sconf.Sconf = sconf.Init(
		*conf_file,
		sconf.Sconf{
         "logtrack_default_log_file": *log_file,
         "access_log":                *access_log,
         "address":                   *address,
         "port":                      *port,
         "ftpport":                   *ftpport})

	client_conf sconf.Sconf = sconf.New(
		*client_conf_file,
		sconf.Sconf{ // these must be entered in client_config.json
			"Links": map[string]interface{}{
				"client conf":         "/clientconf",
				"statdist":            "/statdist",
				"remote statdist":     "/rs",
				"post stat":           "/post_stat",
				"stdout":              "/stdout",
				"access":              "/access",
				"distribution center": "/dc",
				"root":                "/",
				"ffetcher":            conf["fetcher_index"]}})

	ffetch_conf sconf.Sconf = sconf.New(
		*ffetch_conf_file,
		sconf.Sconf{})
)

var (
	log    = logtrack.New()
	access = logtrack.New()
	stat   = stattrack.New("test distribution center")
	fsh    = http.FileServer(http.Dir("/tmp/static"))
)

func Usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS]\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	flag.PrintDefaults()
	fmt.Fprintf(os.Stderr, "\n")
}

type exit struct{}

func (f exit) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	os.Exit(0);
}

type staticfile string

func (f staticfile) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	//record, _ := dkutils.DeepTypeSprint(r)
	access.P(r.RequestURI, " ", r.Proto, " ", fmt.Sprint(r.Header), "\n")

	err := client_conf.Update(*client_conf_file)
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

func main() {

	// typecheck client_conf and set defaults
	fix_config_links()
	var links map[string]interface{}
	links = client_conf["Links"].(map[string]interface{})

	access.Log_file = conf["access_log"].(string)
	access.To_Stdout = false

	flag.Usage = Usage
	flag.Parse()

	var in staticfile = "index.htm"
	http.Handle(links["root"].(string), in)

	var dc staticfile = "dc.htm"
	http.Handle(links["distribution center"].(string), dc)

	var rs staticfile = "rs.htm"
	http.Handle(links["remote statdist"].(string), rs)

	var cli sconf.HTTPHandler = sconf.HTTPHandler(client_conf)
	http.Handle(links["client conf"].(string), cli)

	var jsm statdist.HTTPHandler
	http.Handle(links["statdist"].(string), jsm)

	var sp statdist.HTTPPostHandler
	http.Handle(links["post stat"].(string), sp)

	var sldh logdist.HTTPHandler = "stdout"
	http.Handle(links["stdout"].(string), sldh)

	var ldh logdist.HTTPHandler = logdist.HTTPHandler(
		conf["logtrack_default_log_file"].(string))
	http.Handle("/logfile", ldh)

	var ah logdist.HTTPHandler = logdist.HTTPHandler(conf["access_log"].(string))
	http.Handle(links["access"].(string), ah)

	var f ffetcher.Ffetcher = make(ffetcher.Ffetcher)
	var fhh ffetcher.HTTPHandler = ffetcher.HTTPHandler(f)
	http.Handle(conf["ffetcher_index"].(string), fhh)

	// when this is called the loop running on the docker image will update
	// the go files and restart the server.
	var e exit
	http.Handle("/update", e)

	log.P("starting http server\n")

	log.P(http.ListenAndServe(
		conf["address"].(string)+":"+conf["port"].(string),
		nil))
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
		stat.Warn("client_conf[\"Links\"] is not type map[string]interface{}." +
			" Making it type map[string]interface{}. Check sconf config file: " +
			*client_conf_file)
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
				"config file: " + *client_conf_file + ". Setting Links[\"" + k +
				"\"] to default: /" + k)
			links[k] = "/" + k
			return
		}
	}
}
