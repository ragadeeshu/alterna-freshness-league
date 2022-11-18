package main

import (
	"encoding/json"
	"io/ioutil"
	"os"

	"github.com/ragadeeshu/alterna-freshness-league/datahandling"
	"github.com/ragadeeshu/alterna-freshness-league/web"
)

func main() {
	var league datahandling.League
	byteValue, err := ioutil.ReadFile("contestants.json")
	if err != nil {
		return
	}
	err = json.Unmarshal(byteValue, &league)
	if err != nil {
		panic(err)
	}

	var port string
	if len(os.Args) > 1 {
		port = os.Args[1]
	} else {
		port = "8080"
	}

	cache := datahandling.NewCache()
	web.StartWebServer(cache, &league, port)
}
