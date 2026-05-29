package datahandling

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/patrickmn/go-cache"
)

type LeagueDataCache struct {
	client            *http.Client
	nsoAccountCache   *cache.Cache
	splatnetDataCache *cache.Cache
	sidecarURL        string
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
		client:            client,
		nsoAccountCache:   cache.New(24*time.Hour, 1*time.Hour),
		splatnetDataCache: cache.New(1*time.Minute, 30*time.Second),
		sidecarURL:        sidecarURL,
	}
}

func (c *LeagueDataCache) GetLeagueData(league League) (LeagueResult, error) {
	splatnetDatas, err := c.GetSplatnetDatas(league)
	if err != nil {
		return LeagueResult{}, err
	}
	return calculateResults(splatnetDatas), nil
}

func (c *LeagueDataCache) GetSplatnetDatas(league League) ([]*SplatnetData, error) {
	splatnetDatas := []*SplatnetData{}
	splatnetDataChannel := make(chan *SplatnetData)
	var wg sync.WaitGroup
	for _, contestant := range league.Contestants {
		wg.Add(1)
		go getSplatnetDataAsync(contestant, c.splatnetDataCache, c.nsoAccountCache, c.sidecarURL, splatnetDataChannel, &wg, c.client)
	}
	for _, proxy := range league.Proxies {
		wg.Add(1)
		go getSplatnetProxyAsync(proxy, splatnetDataChannel, &wg, c.client)
	}
	go func() {
		wg.Wait()
		close(splatnetDataChannel)
	}()
	for data := range splatnetDataChannel {
		splatnetDatas = append(splatnetDatas, data)
	}
	return splatnetDatas, nil
}

func getSplatnetDataAsync(
	contestant Contestant,
	splatnetDataCache *cache.Cache,
	nsoAccountCache *cache.Cache,
	sidecarURL string,
	splatnetDataChannel chan *SplatnetData,
	wg *sync.WaitGroup,
	client *http.Client,
) {
	defer wg.Done()

	if cached, found := splatnetDataCache.Get(contestant.Name); found {
		splatnetDataChannel <- cached.(*SplatnetData)
		return
	}

	account, ok := nsoAccountCache.Get(contestant.Name)
	if !ok {
		fetched, err := fetchNsoAccount(&contestant, sidecarURL, client)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to fetch NSO account for contestant %s: %w", contestant.Name, err))
			return
		}
		nsoAccountCache.Add(contestant.Name, fetched, cache.DefaultExpiration)
		account = fetched
	}

	data, err := fetchSplatnetData(contestant.SessionToken, account.(*nsoAccount), sidecarURL, client)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to get SplatNet data for contestant %s: %w", contestant.Name, err))
		return
	}
	splatnetDataCache.Add(contestant.Name, data, cache.DefaultExpiration)
	splatnetDataChannel <- data
}

func getSplatnetProxyAsync(
	proxy string,
	splatnetDataChannel chan *SplatnetData,
	wg *sync.WaitGroup,
	client *http.Client,
) {
	defer wg.Done()
	req, err := http.NewRequest("GET", proxy, nil)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to create request: %w", err))
	}
	response, err := client.Do(req)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to get from proxy %s: %w", proxy, err))
		return
	}
	var proxyDatas []SplatnetData
	err = json.NewDecoder(response.Body).Decode(&proxyDatas)
	if err != nil {
		fmt.Println(fmt.Errorf("failed to unmarshal data from proxy %s: %w", proxy, err))
		return
	}
	for _, data := range proxyDatas {
		splatnetData := data
		splatnetDataChannel <- &splatnetData
	}
}
