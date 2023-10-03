package datahandling

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const heroHistoryQuery = "71019ce4389463d9e2a71632e111eb453ca528f4f794aefd861dff23d9c18147"
const historyRecordQuery = "0a62c0152f27c4218cf6c87523377521c2cff76a4ef0373f2da3300079bf0388"
const graphQlURL = "https://api.lp1.av5ja.srv.nintendo.net/api/graphql"

type historyRecordQueryResponse struct {
	Data struct {
		CurrentPlayer struct {
			ByName    string `json:"byname"`
			Name      string `json:"name"`
			NameId    string `json:"nameId"`
			NamePlate struct {
				Badges []struct {
					Image struct {
						URL string `json:"url"`
					} `json:"image"`
				} `json:"badges"`
				Background struct {
					TextColor struct {
						A float64 `json:"a"`
						R float64 `json:"r"`
						G float64 `json:"g"`
						B float64 `json:"b"`
					}
					Image struct {
						URL string `json:"url"`
					} `json:"image"`
				} `json:"background"`
			} `json:"nameplate"`
		} `json:"currentPlayer"`
	} `json:"data"`
}

type heroHistoryQueryResponse struct {
	Data struct {
		HeroRecord struct {
			ProgressPhrase  string  `json:"progressPhrase"`
			ProgressComment string  `json:"progressComment"`
			ProgressRate    float64 `json:"progressRate"`
			Sites           []struct {
				SiteNumber   int     `json:"siteNumber"`
				ProgressRate float64 `json:"progressRate"`
				SiteName     string  `json:"siteName"`
				Image        struct {
					URL string `json:"url"`
				} `json:"image"`
				ClearedStages []struct {
					StageNumber      int     `json:"stageNumber"`
					IsBoss           bool    `json:"isBoss"`
					BestClearTimeSec float64 `json:"bestClearTimeSec"`
					StageName        string  `json:"stageName"`
					Description      string  `json:"description"`
					BestClearWeapon  struct {
						Name     string `json:"name"`
						Category string `json:"category"`
						Image    struct {
							URL string `json:"url"`
						} `json:"image"`
					} `json:"bestClearWeapon"`
				} `json:"clearedStages"`
			} `json:"sites"`
		} `json:"heroRecord"`
	} `json:"data"`
}

type SplatnetData struct {
	NsoName       string                     `json:"nsoName"`
	NsoImageUrl   string                     `json:"nsoImageUrl"`
	HistoryRecord historyRecordQueryResponse `json:"historyRecord"`
	HeroRecord    heroHistoryQueryResponse   `json:"heroRecord"`
}

func getSplatnetData(account *splatnetAccount, client *http.Client) (*SplatnetData, error) {
	var historyRecordResponse historyRecordQueryResponse
	err := doGraphQlQuery(historyRecordQuery, account, &historyRecordResponse, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get player history: %w", err)
	}
	var heroHistoryResponse heroHistoryQueryResponse
	err = doGraphQlQuery(heroHistoryQuery, account, &heroHistoryResponse, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get hero mode history: %w", err)
	}
	return &SplatnetData{
		NsoName:       account.nsoName,
		NsoImageUrl:   account.nsoImage,
		HistoryRecord: historyRecordResponse,
		HeroRecord:    heroHistoryResponse,
	}, nil
}

func doGraphQlQuery(query string, account *splatnetAccount, jsonTarget interface{}, client *http.Client) error {
	body := map[string]interface{}{
		"extensions": map[string]interface{}{
			"persistedQuery": map[string]interface{}{
				"sha256Hash": query,
				"version":    1,
			},
		},
		"variables": map[string]interface{}{},
	}
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("failed to marshal jsonbody: %w", err)
	}
	req, err := http.NewRequest(http.MethodPost, graphQlURL, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	for k, v := range account.graphQlHeader {
		req.Header.Set(k, v)
	}
	req.AddCookie(&http.Cookie{
		Name:  "_gtoken",
		Value: account.accessToken,
	})
	response, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer response.Body.Close()
	err = json.NewDecoder(response.Body).Decode(&jsonTarget)
	if err != nil {
		return fmt.Errorf("failed to decode json from response: %w", err)
	}
	return nil
}
