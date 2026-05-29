package main

import (
	"encoding/json"
	"flag"
	"os"
	"strconv"

	"github.com/google/go-cmp/cmp"
	"github.com/ragadeeshu/alterna-freshness-league/datahandling"
	"github.com/ragadeeshu/alterna-freshness-league/web"
)

func main() {
	var league datahandling.League
	port := flag.String("port", envString("PORT", "8080"), "Port to bind to")
	proxy := flag.Bool("proxy", envBool("PROXY", false), "Start in proxy mode")
	sidecarURL := flag.String("sidecar-url", envString("SIDECAR_URL", "http://127.0.0.1:2727"), "Base URL for the nxapi-bridge sidecar")
	flag.Var(&league, "league", "Contestants json string; falls back to CONTESTANTS env var, then 'contestants.json'")
	flag.Parse()

	if cmp.Equal(league, datahandling.League{}) {
		if raw := os.Getenv("CONTESTANTS"); raw != "" && raw != "{}" {
			if err := json.Unmarshal([]byte(raw), &league); err != nil {
				panic(err)
			}
		} else {
			byteValue, err := os.ReadFile("contestants.json")
			if err != nil {
				return
			}
			if err := json.Unmarshal(byteValue, &league); err != nil {
				panic(err)
			}
		}
	}

	cache := datahandling.NewCache(*sidecarURL)
	if *proxy {
		web.StartProxyServer(cache, league, *port)
	} else {
		web.StartWebServer(cache, league, *port)
	}
}

func envString(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return fallback
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return fallback
	}
	return b
}
