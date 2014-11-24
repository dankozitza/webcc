package main

import (
   "fmt"
   "github.com/dankozitza/sconf"
   "github.com/dankozitza/stattrack"
   "github.com/dankozitza/statdist"
   "github.com/dankozitza/logtrack"
   "github.com/dankozitza/logdist"
   "net/http"
   "os"
   "io"
)

var log_file string = "cc.log"

var conf sconf.Sconf = sconf.New("config.json",
   sconf.Sconf{"logtrack_default_log_file" : log_file})

var log logtrack.LogTrack = logtrack.New()

var stat stattrack.StatTrack = stattrack.New("test control center")

type htmfile string

func (f htmfile) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
}

func main() {

   var jsm statdist.JSONStatMap
   http.Handle("/statdist", jsm)

   var sldh logdist.LogDistHandler = "stdout"
   http.Handle("/stdout", sldh)

   var ldh logdist.LogDistHandler = logdist.LogDistHandler(log_file)
   http.Handle("/logfile", ldh)

   var hf htmfile = "index.htm"
   http.Handle("/cc", hf)

   log.P("starting http server")

   http.ListenAndServe("localhost:9000", nil)
   return
}
