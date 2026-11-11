package web

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"

	"github.com/ragadeeshu/alterna-freshness-league/datahandling"
)

var funcMap = template.FuncMap{
	"upper": strings.ToUpper,
	"badgeinc": func(f float64) float64 {
		return f + 74
	},
	"arrayIndex": func(i int) int {
		return i - 1
	},
	"siteWinner": func(results []datahandling.PlayerResult, siteNumber int) string {
		var winners []string
		for _, result := range results {
			if site, found := result.ResultBySite[siteNumber]; found && site.Rank == 1 {
				winners = append(winners, result.Name)
			}
		}
		if len(winners) == 0 {
			return "Not found"
		}
		return strings.Join(winners, " & ")
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
	"formatTime": datahandling.FormatTime,
	// keyIcon picks SplatNet's palette key icon (2 unlocked, 1 unlocking,
	// 0 locked); Eight's palette has its own set. Loaded from SplatNet.
	"keyIcon": func(status int, hachi bool) string {
		icons := map[int]string{
			2: "iconUnlocked.12faa13030d04549e9f053e94f816ae5.svg",
			1: "iconUnlocking.c69cac01e3eb815becfa409b72cfb64d.svg",
			0: "iconLocked.47069cc538b925a2b0b6b480dccc3ad8.svg",
		}
		if hachi {
			icons = map[int]string{
				2: "iconUnlocked8.9149a961fa0c5c90fbcb5382b92aa21b.svg",
				1: "iconUnlocking8.e0a40d96b6fe59da9e28521fc62d2309.svg",
				0: "iconLocked8.ac083011bb7dfa15da6a77cc9f5616e2.svg",
			}
		}
		icon, found := icons[status]
		if !found {
			icon = icons[0]
		}
		return "https://api.lp1.av5ja.srv.nintendo.net/static/media/" + icon
	},
	// chipWheel draws SplatNet's color chip usage donut as a conic gradient,
	// coloured by --color-chip-<group> (defined in ordercss.css).
	"chipWheel": func(usage []datahandling.OrderChipUsage) string {
		total := 0.0
		for _, u := range usage {
			total += u.Rate
		}
		if total <= 0 {
			return "none"
		}
		var stops []string
		start := 0.0
		for _, u := range usage {
			end := start + u.Rate/total*100
			stops = append(stops, fmt.Sprintf("var(--color-chip-%s) %.2f%% %.2f%%", u.Group, start, end))
			start = end
		}
		return "conic-gradient(" + strings.Join(stops, ", ") + ")"
	},
	"chipPercent": func(rate float64) string {
		return fmt.Sprintf("%.1f%%", rate*100)
	},
	"thousands": func(n int) string {
		s := strconv.Itoa(n)
		for i := len(s) - 3; i > 0; i -= 3 {
			s = s[:i] + "," + s[i:]
		}
		return s
	},
	"ordinalSuffix": func(n int) string {
		switch {
		case n%100 >= 11 && n%100 <= 13:
			return "th"
		case n%10 == 1:
			return "st"
		case n%10 == 2:
			return "nd"
		case n%10 == 3:
			return "rd"
		}
		return "th"
	},
}

var leagueTemplate = template.Must(
	template.New("league.gohtml").Funcs(funcMap).ParseFiles("./web/content/league.gohtml"),
)

var orderTemplate = template.Must(
	template.New("order.gohtml").Funcs(funcMap).ParseFiles("./web/content/order.gohtml"),
)

func StartWebServer(cache *datahandling.LeagueDataCache, league datahandling.League, port string) {
	handler := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/league" {
			http.Error(w, "404 not found.", http.StatusNotFound)
			return
		}

		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}

		if r.URL.Path == "/league" {
			results, err := cache.GetLeagueData(league)
			if err != nil {
				fmt.Println(err)
			}
			if err := leagueTemplate.Execute(w, results); err != nil {
				fmt.Println(err)
			}
		} else {
			body, _ := os.ReadFile("./web/content/index.html")
			fmt.Fprintf(w, "%s", body)
		}
	}

	// Side Order league: /order is the loading shell, which fetches the
	// rendered page from /order/league.
	orderHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}
		switch r.URL.Path {
		case "/order":
			body, _ := os.ReadFile("./web/content/order.html")
			fmt.Fprintf(w, "%s", body)
		case "/order/":
			http.Redirect(w, r, "../order", http.StatusMovedPermanently)
		case "/order/league":
			results, err := cache.GetOrderLeagueData(league)
			if err != nil {
				fmt.Println(err)
			}
			// Every run's color chip + hacks summary is embedded for the
			// overlay, so the page is large but repetitive: compress it.
			out, done := gzipWriter(w, r)
			if err := orderTemplate.Execute(out, results); err != nil {
				fmt.Println(err)
			}
			done()
		default:
			http.Error(w, "404 not found.", http.StatusNotFound)
		}
	}

	fileServer := http.FileServer(http.Dir("./web/content/static"))
	http.HandleFunc("/", handler)
	http.HandleFunc("/order", orderHandler)
	http.HandleFunc("/order/", orderHandler)
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

	// Side Order data for the main instance's /order page.
	orderHandler := func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method is not supported.", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		sideOrderDataPointers, err := cache.GetSideOrderDatas(league)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		sideOrderDatas := []datahandling.SideOrderData{}
		for _, sideOrderData := range sideOrderDataPointers {
			sideOrderDatas = append(sideOrderDatas, *sideOrderData)
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(sideOrderDatas)
	}

	http.HandleFunc("/", handler)
	http.HandleFunc("/order", orderHandler)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}

// gzipWriter returns a writer that gzips the response when the client accepts
// it, and a func to call once everything has been written.
func gzipWriter(w http.ResponseWriter, r *http.Request) (io.Writer, func()) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Add("Vary", "Accept-Encoding")
	if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
		return w, func() {}
	}
	w.Header().Set("Content-Encoding", "gzip")
	gz := gzip.NewWriter(w)
	return gz, func() { gz.Close() }
}
