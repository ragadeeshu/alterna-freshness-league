package datahandling

import (
	"fmt"
	"net/http"
	"time"
)

type Cache struct {
	client *http.Client
}

type splatnetAccount struct {
	Nickname      string
	Image         string
	AccessToken   string
	GraphQlHeader map[string]string
}
type League struct {
	LeagueName  string       `json:"league"`
	Contestants []Contestant `json:"contestants"`
}

type Contestant struct {
	Name         string `json:"name"`
	SessionToken string `json:"session_token"`
}

func NewCache() *Cache {
	var client = &http.Client{
		Timeout: time.Second * 30,
	}
	return &Cache{
		client: client,
	}
}

func (c *Cache) GetLeagueData(league *League) (interface{}, error) {
	webViewVersion, err := getWebViewVersion(c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get webview version: %w", err)
	}
	nsoAppVersion, err := getNsoAppVersion(c.client)
	if err != nil {
		return nil, fmt.Errorf("failed to get nso app version: %w", err)
	}
	splatnetData := []*splatnetData{}
	for _, contestant := range league.Contestants {
		account, err := splatnetLogin(&contestant, nsoAppVersion, webViewVersion, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to log in contestant %s: %w", contestant.Name, err)
		}
		data, err := getSplatnetData(account, c.client)
		if err != nil {
			return nil, fmt.Errorf("failed to get data for contestant %s: %w", contestant.Name, err)
		}
		splatnetData = append(splatnetData, data)
	}
	return splatnetData, nil
}
