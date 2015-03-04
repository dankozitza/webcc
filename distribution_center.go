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
	"time"
)

var (
	address    = flag.String("h", "localhost", "ip address to host on")
	port       = flag.String("p", "9000", "port to host on")
	ftpport    = flag.String("f", "9001", "port to host ftp on")
	log_file   = flag.String("l", "dc.log", "log file to print to")
	access_log = flag.String(
		"a", "dc_access.log", "log file to print http logs")
	ffetch_conf_file = flag.String(
		"fc", "ffetch_config.json", "json config file for ffetch")
	client_conf_file = flag.String(
		"cc", "client_config.json", "json config file for client")
	conf_file = flag.String("c", "config.json", "json config file for server")
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
		sconf.Sconf{
			"Links": map[string]interface{}{
				"client conf":         "/clientconf",
				"statdist":            "/statdist",
				"stdout":              "/stdout",
				"access":              "/access",
				"distribution center": "/dc",
				"file server":         "http://localhost:9001",
				"ffetcher":            "/fetcher"}})

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

type staticfile string

func (f staticfile) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	access.P(r.RemoteAddr, " ", r, "\n")

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

	access.Log_file = conf["access_log"].(string)
	access.To_Stdout = false

	flag.Usage = Usage
	flag.Parse()

	var fsh myFileServer
	s := &http.Server{
		Addr:           conf["address"].(string) + ":" + conf["ftpport"].(string),
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

	var ldh logdist.HTTPHandler = logdist.HTTPHandler(
		conf["logtrack_default_log_file"].(string))
	http.Handle("/logfile", ldh)

	var ah logdist.HTTPHandler = logdist.HTTPHandler(conf["access_log"].(string))
	http.Handle(links["access"].(string), ah)

	var in staticfile = "index.htm"
	http.Handle(links["distribution center"].(string), in)

	var f ffetcher.Ffetcher = make(ffetcher.Ffetcher)
	//go ffetcher.Crawl(conf["ffetch_url"].(string), int(conf["ffetch_depth"].(float64)), f)

	//ffetch_conf["ffetcher"] = f
	var fhh ffetcher.HTTPHandler = ffetcher.HTTPHandler(f)

	http.Handle(links["ffetcher"].(string), fhh)

	//for u, _ := range f {

	//	client_conf["ffetcher"].(map[string]*ffetcher.Fresult)[u] = f[u]
	//	//<-fetcher[u].done
	//	//fmt.Println("fetcher[", u, "] = {\n body\n urls = [")
	//	//for _, s := range fetcher[u].urls {
	//	//	fmt.Println("", s, ",")
	//	//}
	//	//fmt.Println(" ]\n}")
	//}

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
