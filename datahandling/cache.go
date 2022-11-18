package datahandling

import (
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

func (c *LeagueDataCache) GetLeagueData(league *League) (LeagueResult, error) {

	webViewVersionChannel := make(chan string)
	nsoAppVersionChannel := make(chan string)
	errorChannel := make(chan error)

	go getVersionAsync(c.versionCache, "webViewVersion", func() (string, error) { return getWebViewVersion(c.client) }, webViewVersionChannel, errorChannel)
	go getVersionAsync(c.versionCache, "nsoAppVersion", func() (string, error) { return getNsoAppVersion(c.client) }, nsoAppVersionChannel, errorChannel)

	var webViewVersion string
	select {
	case webViewVersion = <-webViewVersionChannel:
	case err := <-errorChannel:
		return LeagueResult{}, err
	}

	var nsoAppVersion string
	select {
	case nsoAppVersion = <-nsoAppVersionChannel:
	case err := <-errorChannel:
		return LeagueResult{}, err
	}

	splatnetDatas := []*splatnetData{}
	splatnetDataChannel := make(chan *splatnetData)
	var wg sync.WaitGroup
	for _, contestant := range league.Contestants {
		wg.Add(1)
		go getSplatnetDataAsync(contestant, c.splatnetDataCache, c.splatnetAccountCache, nsoAppVersion, webViewVersion, splatnetDataChannel, &wg, c.client)
	}
	go func() {
		wg.Wait()
		close(splatnetDataChannel)
	}()
	for data := range splatnetDataChannel {
		splatnetDatas = append(splatnetDatas, data)
	}
	return calculateResults(splatnetDatas), nil
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
	splatnetDataChannel chan *splatnetData,
	wg *sync.WaitGroup,
	client *http.Client,
) {
	defer wg.Done()
	var err error
	var data *splatnetData
	if cacheValue, found := splatnetDataCache.Get(contestant.Name); found {
		data = cacheValue.(*splatnetData)
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
