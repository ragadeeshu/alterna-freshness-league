package datahandling

import (
	"sort"
	"strings"
)

var freshness = [...]string{"Dry", "Raw", "Fresh", "Smokin'", "Profreshional", "SUPERFRESH!"}

type World struct {
	Sites []Site
}

type Site struct {
	SiteNumber int
	SiteName   string
	ImageURL   string
	Stages     []Stage
}

type Stage struct {
	StageNumber int
	IsBoss      bool
	StageName   string
	Description string
}

type PlayerResult struct {
	ByName             string
	Name               string
	NameId             string
	NamePlate          NamePlate
	TotalScore         int
	Rank               int
	ExplorationRate    float64
	ExplorationPhrase  string
	ExplorationComment string
	Freshness          string
	BestSite           int
	WorstSite          int
	ResultBySite       map[int]*SiteResult
}

type NamePlate struct {
	BadgeURLs          []string
	TextA              float64
	TextR              float64
	TextG              float64
	TextB              float64
	BackgroundImageURL string
}

type SiteResult struct {
	Score         int
	Rank          int
	ResultByStage map[int]*StageResult
}

type StageResult struct {
	Score          int
	Rank           int
	Time           float64
	WeaponName     string
	WeaponCategory string
	WeaponImageURL string
}

func calculateResults(splatnetDatas []*splatnetData) *LeagueResult {
	worldView := gatherWorldView(splatnetDatas)
	playerResults := gatherPlayerTimes(splatnetDatas)
	score(playerResults, &worldView)
	return &LeagueResult{
		World:         worldView,
		PlayerResults: *playerResults,
	}
}

func gatherWorldView(splatnetDatas []*splatnetData) (world World) {
	sites := make(map[int]*Site)
	siteStages := make(map[int]map[int]*Stage)
	for _, splatnetData := range splatnetDatas {
		for _, splatnetSite := range splatnetData.heroRecord.Data.HeroRecord.Sites {
			// find all sites from this data
			if _, found := sites[splatnetSite.SiteNumber]; !found {
				sites[splatnetSite.SiteNumber] = &Site{
					SiteNumber: splatnetSite.SiteNumber,
					SiteName:   splatnetSite.SiteName,
					ImageURL:   splatnetSite.Image.URL,
				}
				siteStages[splatnetSite.SiteNumber] = make(map[int]*Stage)
			}
			// find all stages from this data
			for _, splatnetStage := range splatnetSite.ClearedStages {
				if _, found := siteStages[splatnetSite.SiteNumber][splatnetStage.StageNumber]; !found {
					siteStages[splatnetSite.SiteNumber][splatnetStage.StageNumber] = &Stage{
						StageNumber: splatnetStage.StageNumber,
						IsBoss:      splatnetStage.IsBoss,
						StageName:   splatnetStage.StageName,
						Description: splatnetStage.Description,
					}
				}
			}
		}
	}
	// sort and add stages
	for _, site := range sites {
		for _, stage := range siteStages[site.SiteNumber] {
			site.Stages = append(site.Stages, *stage)
		}
		sort.Slice(site.Stages, func(i, j int) bool { return site.Stages[i].StageNumber < site.Stages[j].StageNumber })
		//add site
		world.Sites = append(world.Sites, *site)
	}
	//sort sites
	sort.Slice(world.Sites, func(i, j int) bool { return world.Sites[i].SiteNumber < world.Sites[j].SiteNumber })
	return
}

func gatherPlayerTimes(splatnetDatas []*splatnetData) *[]PlayerResult {
	playerResults := []PlayerResult{}
	for _, playerData := range splatnetDatas {
		badgeURLs := []string{}
		for _, badge := range playerData.historyRecord.Data.CurrentPlayer.NamePlate.Badges {
			badgeURLs = append(badgeURLs, badge.Image.URL)
		}
		playerResult := PlayerResult{
			ByName: playerData.historyRecord.Data.CurrentPlayer.ByName,
			Name:   playerData.historyRecord.Data.CurrentPlayer.Name,
			NameId: playerData.historyRecord.Data.CurrentPlayer.NameId,
			NamePlate: NamePlate{
				BadgeURLs:          badgeURLs,
				TextA:              playerData.historyRecord.Data.CurrentPlayer.NamePlate.Background.TextColor.A,
				TextR:              playerData.historyRecord.Data.CurrentPlayer.NamePlate.Background.TextColor.R,
				TextG:              playerData.historyRecord.Data.CurrentPlayer.NamePlate.Background.TextColor.G,
				TextB:              playerData.historyRecord.Data.CurrentPlayer.NamePlate.Background.TextColor.B,
				BackgroundImageURL: playerData.historyRecord.Data.CurrentPlayer.NamePlate.Background.Image.URL,
			},
			ExplorationRate:    playerData.heroRecord.Data.HeroRecord.ProgressRate,
			ExplorationPhrase:  playerData.heroRecord.Data.HeroRecord.ProgressPhrase,
			ExplorationComment: playerData.heroRecord.Data.HeroRecord.ProgressComment,
			ResultBySite:       map[int]*SiteResult{},
		}
		for _, site := range playerData.heroRecord.Data.HeroRecord.Sites {
			playerResult.ResultBySite[site.SiteNumber] = &SiteResult{
				ResultByStage: map[int]*StageResult{},
			}
			for _, stage := range site.ClearedStages {
				playerResult.ResultBySite[site.SiteNumber].ResultByStage[stage.StageNumber] = &StageResult{
					Time:           stage.BestClearTimeSec,
					WeaponName:     strings.Replace(stage.BestClearWeapon.Name, `ヒーローシューター`, `Hero shot `, 1),
					WeaponCategory: stage.BestClearWeapon.Category,
					WeaponImageURL: stage.BestClearWeapon.Image.URL,
				}
			}
		}
		playerResults = append(playerResults, playerResult)
	}
	return &playerResults
}

func score(playerResults *[]PlayerResult, worldView *World) {
	maxScore := len(*playerResults)
	numStages := 0
	for _, site := range worldView.Sites {
		// score stages
		for _, stage := range site.Stages {
			numStages++
			var stageResultPointers []*StageResult
			// find stage result, if any
			for _, playerResult := range *playerResults {
				if siteResult, found := playerResult.ResultBySite[site.SiteNumber]; found {
					if stageResult, found := siteResult.ResultByStage[stage.StageNumber]; found {
						stageResultPointers = append(stageResultPointers, stageResult)
					}
				}
			}
			sort.Slice(stageResultPointers, func(i, j int) bool { return stageResultPointers[i].Time < stageResultPointers[j].Time })
			// award points and ranks
			for i, stageResultPointer := range stageResultPointers {
				stageResultPointer.Rank = i + 1
				stageResultPointer.Score = maxScore - i
			}
		}
		// score site
		var siteResultPointers []*SiteResult
		// find site result, if any
		for _, playerResult := range *playerResults {
			if siteResult, found := playerResult.ResultBySite[site.SiteNumber]; found {
				siteResultPointers = append(siteResultPointers, siteResult)
			}
		}
		// award points
		for _, siteResultPointer := range siteResultPointers {
			for _, stageResult := range siteResultPointer.ResultByStage {
				siteResultPointer.Score += stageResult.Score
			}
		}
		sort.Slice(siteResultPointers, func(i, j int) bool { return siteResultPointers[i].Score > siteResultPointers[j].Score })
		// award ranks
		for index, siteResultPointer := range siteResultPointers {
			siteResultPointer.Rank = index + 1
		}
	}
	// total score
	for i := range *playerResults {
		for _, site := range worldView.Sites {
			if siteResult, found := (*playerResults)[i].ResultBySite[site.SiteNumber]; found {
				(*playerResults)[i].TotalScore += siteResult.Score
			}
		}
	}
	sort.Slice(*playerResults, func(i, j int) bool { return (*playerResults)[i].TotalScore > (*playerResults)[j].TotalScore })
	freshnessThreshold := numStages * maxScore / (len(freshness) - 1)
	for i := range *playerResults {
		(*playerResults)[i].Rank = i + 1
		(*playerResults)[i].Freshness = freshness[(*playerResults)[i].TotalScore/freshnessThreshold]
		(*playerResults)[i].BestSite = 1
		(*playerResults)[i].WorstSite = 5
		bestSiteRank := len(*playerResults) + 1
		worstSiteRank := 1
		for siteNumber, site := range (*playerResults)[i].ResultBySite {
			if site.Rank < bestSiteRank || site.Rank == bestSiteRank && siteNumber < (*playerResults)[i].BestSite {
				bestSiteRank = site.Rank
				(*playerResults)[i].BestSite = siteNumber
			}
			if site.Rank > worstSiteRank || site.Rank == worstSiteRank && siteNumber > (*playerResults)[i].WorstSite {
				worstSiteRank = site.Rank
				(*playerResults)[i].WorstSite = siteNumber
			}
		}
	}
}
