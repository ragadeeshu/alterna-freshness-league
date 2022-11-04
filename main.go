package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/ragadeeshu/alterna-freshness-league/datahandling"
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

	cache := datahandling.NewCache()
	data, err := cache.GetLeagueData(&league)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Printf("%+v", data)
}
