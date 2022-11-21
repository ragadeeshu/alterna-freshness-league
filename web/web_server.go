package web

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"net/http"
	"text/template"

	"github.com/ragadeeshu/alterna-freshness-league/datahandling"
)

func StartWebServer(cache *datahandling.LeagueDataCache, league datahandling.League, port string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}
		results, err := cache.GetLeagueData(league)
		if err != nil {
			fmt.Println(err)
		}

		funcMap := template.FuncMap{
			"badgeinc": func(f float64) float64 {
				return f + 74
			},
			"arrayIndex": func(i int) int {
				return i - 1
			},
			"siteWinner": func(results []datahandling.PlayerResult, siteNumber int) string {
				for _, result := range results {
					if site, found := result.ResultBySite[siteNumber]; found && site.Rank == 1 {
						return result.Name
					}
				}
				return "Not found"
			},
			"siteScore": func(result datahandling.PlayerResult, siteNumber int) int {
				if site, found := result.ResultBySite[siteNumber]; found {
					return site.Score
				}
				return 0
			},
			"siteRank": func(result datahandling.PlayerResult, siteNumber int) int {
				if site, found := result.ResultBySite[siteNumber]; found {
					return site.Rank
				}
				return 0
			},
			"hasStageResult": func(result datahandling.PlayerResult, siteNumber int, stageNumber int) bool {
				if site, found := result.ResultBySite[siteNumber]; found {
					_, found = site.ResultByStage[stageNumber]
					return found
				}
				return false
			},
			"percentageFormat": func(f float64) string {
				return fmt.Sprintf("%.0f%%", f*100)
			},
			"formatTime": func(time float64) string {
				return fmt.Sprintf("%02d:%05.2f", int(time/60), math.Mod(time, 60))
			},
		}

		t, err := template.New("league.gohtml").Funcs(funcMap).ParseFiles("./web/content/league.gohtml")
		if err != nil {
			fmt.Println(err)
		}
		err = t.Execute(w, results)
		if err != nil {
			fmt.Println(err)
		}
	}

	fileServer := http.FileServer(http.Dir("./web/content/static"))
	http.HandleFunc("/", handler)
	http.Handle("/static/", http.StripPrefix("/static", fileServer))
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

func StartProxyServer(cache *datahandling.LeagueDataCache, league datahandling.League, port string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		splatnetDataPointers, err := cache.GetSplatnetDatas(league)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		var splatnetDatas []datahandling.SplatnetData
		for _, splatnetData := range splatnetDataPointers {
			splatnetDatas = append(splatnetDatas, *splatnetData)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(splatnetDatas)
	}

	http.HandleFunc("/", handler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
