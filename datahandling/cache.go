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
	client               *http.Client
	versionCache         *cache.Cache
	splatnetAccountCache *cache.Cache
	splatnetDataCache    *cache.Cache
}

type splatnetAccount struct {
	nsoName       string
	nsoImage      string
	accessToken   string
	graphQlHeader map[string]string
}
type League struct {
	LeagueName  string       `json:"league"`
	Contestants []Contestant `json:"contestants"`
	Proxies     []string     `json:"proxies"`
}

func (this *League) String() string {
	b, _ := json.Marshal(*this)
	return string(b)
}

func (this *League) Set(s string) error {
	return json.Unmarshal([]byte(s), this)
}

type Contestant struct {
	Name         string `json:"name"`
	SessionToken string `json:"session_token"`
}

type LeagueResult struct {
	World         World
	PlayerResults []PlayerResult
}

func NewCache() *LeagueDataCache {
	var client = &http.Client{
		Timeout: time.Second * 30,
	}
	return &LeagueDataCache{
		client:               client,
		versionCache:         cache.New(24*time.Hour, 1*time.Hour),
		splatnetAccountCache: cache.New(109*time.Minute, 10*time.Minute),
		splatnetDataCache:    cache.New(1*time.Minute, 30*time.Second),
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
	webViewVersionChannel := make(chan string)
	nsoAppVersionChannel := make(chan string)
	errorChannel := make(chan error)

	go getVersionAsync(c.versionCache, "webViewVersion", func() (string, error) { return getWebViewVersion(c.client) }, webViewVersionChannel, errorChannel)
	go getVersionAsync(c.versionCache, "nsoAppVersion", func() (string, error) { return getNsoAppVersion(c.client) }, nsoAppVersionChannel, errorChannel)

	var webViewVersion string
	select {
	case webViewVersion = <-webViewVersionChannel:
	case err := <-errorChannel:
		return nil, err
	}

	var nsoAppVersion string
	select {
	case nsoAppVersion = <-nsoAppVersionChannel:
	case err := <-errorChannel:
		return nil, err
	}

	splatnetDatas := []*SplatnetData{}
	splatnetDataChannel := make(chan *SplatnetData)
	var wg sync.WaitGroup
	for _, contestant := range league.Contestants {
		wg.Add(1)
		go getSplatnetDataAsync(contestant, c.splatnetDataCache, c.splatnetAccountCache, nsoAppVersion, webViewVersion, splatnetDataChannel, &wg, c.client)
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

func getVersionAsync(versionCache *cache.Cache, versionType string, fetcher func() (string, error), versionChannel chan string, errorChannel chan error) {
	if cacheValue, found := versionCache.Get(versionType); found {
		versionChannel <- cacheValue.(string)
		return
	}
	version, err := fetcher()
	if err != nil {
		errorChannel <- fmt.Errorf("failed to get %s: %w", versionType, err)
		return
	}
	err = versionCache.Add(versionType, version, cache.DefaultExpiration)
	if err != nil {
		errorChannel <- fmt.Errorf("failed to cache %s: %w", versionType, err)
		return
	}
	versionChannel <- version
}

func getSplatnetDataAsync(
	contestant Contestant,
	splatnetDataCache *cache.Cache,
	splatnetAccountCache *cache.Cache,
	nsoAppVersion string,
	webViewVersion string,
	splatnetDataChannel chan *SplatnetData,
	wg *sync.WaitGroup,
	client *http.Client,
) {
	defer wg.Done()
	var err error
	var data *SplatnetData
	if cacheValue, found := splatnetDataCache.Get(contestant.Name); found {
		data = cacheValue.(*SplatnetData)
	} else {
		var account *splatnetAccount
		if cacheValue, found := splatnetAccountCache.Get(contestant.Name); found {
			account = cacheValue.(*splatnetAccount)
		} else {
			account, err = splatnetLogin(&contestant, nsoAppVersion, webViewVersion, client)
			if err != nil {
				fmt.Println(fmt.Errorf("failed to log in contestant %s: %w", contestant.Name, err))
				return
			}
			splatnetAccountCache.Add(contestant.Name, account, cache.DefaultExpiration)
		}
		data, err = getSplatnetData(account, client)
		if err != nil {
			fmt.Println(fmt.Errorf("failed to get data for contestant %s: %w", contestant.Name, err))
			return
		}
		splatnetDataCache.Add(contestant.Name, data, cache.DefaultExpiration)
	}
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
