package datahandling

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
