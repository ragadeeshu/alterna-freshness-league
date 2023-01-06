package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"

	"github.com/google/go-cmp/cmp"
	"github.com/ragadeeshu/alterna-freshness-league/datahandling"
	"github.com/ragadeeshu/alterna-freshness-league/web"
)

func main() {
	var league datahandling.League
	port := flag.String("port", "8080", "Port to bind to")
	proxy := flag.Bool("proxy", false, "Start in proxy mode")
	flag.Var(&league, "league", "Contestants json string, tries to read 'contestants.json' file if not set")
	flag.String("language", "en-US", "Language to use for the league")
	flag.Parse()

	if cmp.Equal(league, datahandling.League{}) {
		byteValue, err := ioutil.ReadFile("contestants.json")
		if err != nil {
			return
		}
		err = json.Unmarshal(byteValue, &league)
		if err != nil {
			panic(err)
		}
	}
	cache := datahandling.NewCache()
	if *proxy {
		web.StartProxyServer(cache, league, *port)
	} else {
		web.StartWebServer(cache, league, *port)
	}
}
