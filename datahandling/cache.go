package datahandling

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type LeagueDataCache struct {
	client             *http.Client
	nsoAccountCache    *cache.Cache
	splatnetDataCache  *cache.Cache
	sideOrderDataCache *cache.Cache
	sidecarURL         string
}

type League struct {
	Contestants []Contestant `json:"contestants"`
	Proxies     []string     `json:"proxies"`
}

func (l *League) String() string {
	b, _ := json.Marshal(*l)
	return string(b)
}

func (l *League) Set(s string) error {
	return json.Unmarshal([]byte(s), l)
}

type Contestant struct {
	Name         string `json:"name"`
	SessionToken string `json:"session_token"`
}

type LeagueResult struct {
	World         World
	PlayerResults []PlayerResult
}

func NewCache(sidecarURL string) *LeagueDataCache {
	var client = &http.Client{
		Timeout: time.Second * 180,
	}
	return &LeagueDataCache{
		client:             client,
		nsoAccountCache:    cache.New(24*time.Hour, 1*time.Hour),
		splatnetDataCache:  cache.New(1*time.Minute, 30*time.Second),
		sideOrderDataCache: cache.New(1*time.Minute, 30*time.Second),
		sidecarURL:         sidecarURL,
	}
}

func (c *LeagueDataCache) GetLeagueData(league League) (LeagueResult, error) {
	splatnetDatas, err := c.GetSplatnetDatas(league)
	if err != nil {
		return LeagueResult{}, err
	}
	return calculateResults(splatnetDatas), nil
}

func (c *LeagueDataCache) GetOrderLeagueData(league League) (OrderLeagueResult, error) {
	sideOrderDatas, err := c.GetSideOrderDatas(league)
	if err != nil {
		return OrderLeagueResult{}, err
	}
	return calculateOrderResults(sideOrderDatas), nil
}

// GetSplatnetDatas fetches hero/history data for every contestant and proxy
// ("/" on each proxy) concurrently.
func (c *LeagueDataCache) GetSplatnetDatas(league League) ([]*SplatnetData, error) {
	return gatherAll(league, c.client, "/", func(contestant Contestant) *SplatnetData {
		return c.getSplatnetData(contestant)
	}), nil
}

// GetSideOrderDatas fetches Side Order data for every contestant and proxy
// ("/order" on each proxy) concurrently.
func (c *LeagueDataCache) GetSideOrderDatas(league League) ([]*SideOrderData, error) {
	return gatherAll(league, c.client, "/order", func(contestant Contestant) *SideOrderData {
		return c.getSideOrderData(contestant)
	}), nil
}

func gatherAll[T any](league League, client *http.Client, proxyPath string, fetch func(Contestant) *T) []*T {
	results := []*T{}
	resultChannel := make(chan *T)
	var wg sync.WaitGroup
	for _, contestant := range league.Contestants {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if data := fetch(contestant); data != nil {
				resultChannel <- data
			}
		}()
	}
	for _, proxy := range league.Proxies {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, data := range getFromProxy[T](proxy, proxyPath, client) {
				resultChannel <- data
			}
		}()
	}
	go func() {
		wg.Wait()
		close(resultChannel)
	}()
	for data := range resultChannel {
		results = append(results, data)
	}
	return results
}

func (c *LeagueDataCache) getSplatnetData(contestant Contestant) *SplatnetData {
	if cached, found := c.splatnetDataCache.Get(contestant.Name); found {
		return cached.(*SplatnetData)
	}
	account, err := c.getNsoAccount(contestant)
	if err != nil {
		log.Printf("failed to fetch NSO account for contestant %s: %v", contestant.Name, err)
		return nil
	}
	data, err := fetchSplatnetData(contestant.SessionToken, account, c.sidecarURL, c.client)
	if err != nil {
		log.Printf("failed to get SplatNet data for contestant %s: %v", contestant.Name, err)
		return nil
	}
	c.splatnetDataCache.Add(contestant.Name, data, cache.DefaultExpiration)
	return data
}

func (c *LeagueDataCache) getSideOrderData(contestant Contestant) *SideOrderData {
	if cached, found := c.sideOrderDataCache.Get(contestant.Name); found {
		return cached.(*SideOrderData)
	}
	account, err := c.getNsoAccount(contestant)
	if err != nil {
		log.Printf("failed to fetch NSO account for contestant %s: %v", contestant.Name, err)
		return nil
	}
	data, err := fetchSideOrderData(contestant.SessionToken, account, c.sidecarURL, c.client)
	if err != nil {
		log.Printf("failed to get Side Order data for contestant %s: %v", contestant.Name, err)
		return nil
	}
	c.sideOrderDataCache.Add(contestant.Name, data, cache.DefaultExpiration)
	return data
}

// getNsoAccount returns the contestant's NSO name and avatar, cached for a day
// and shared by both pages.
func (c *LeagueDataCache) getNsoAccount(contestant Contestant) (*nsoAccount, error) {
	if account, ok := c.nsoAccountCache.Get(contestant.Name); ok {
		return account.(*nsoAccount), nil
	}
	account, err := fetchNsoAccount(&contestant, c.sidecarURL, c.client)
	if err != nil {
		return nil, err
	}
	c.nsoAccountCache.Add(contestant.Name, account, cache.DefaultExpiration)
	return account, nil
}

// getFromProxy fetches a JSON array of T from a proxy instance. path is "/"
// for SplatNet hero data and "/order" for Side Order data; proxies running an
// older version 404 on "/order" and are skipped.
func getFromProxy[T any](proxy, path string, client *http.Client) []*T {
	url := proxy
	if path != "/" {
		url = strings.TrimRight(proxy, "/") + path
	}
	response, err := client.Get(url)
	if err != nil {
		log.Printf("failed to get from proxy %s: %v", url, err)
		return nil
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		log.Printf("failed to get from proxy %s: %s", url, response.Status)
		return nil
	}
	var proxyDatas []*T
	if err := json.NewDecoder(response.Body).Decode(&proxyDatas); err != nil {
		log.Printf("failed to unmarshal data from proxy %s: %v", url, err)
		return nil
	}
	return proxyDatas
}
