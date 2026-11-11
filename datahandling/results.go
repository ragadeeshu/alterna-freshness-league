package datahandling

import (
	"math"
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

// Profile is the player identity shown on both pages: NSO name/avatar and the
// in-game nameplate.
type Profile struct {
	NsoImageUrl string
	NsoName     string
	ByName      string
	Name        string
	NameId      string
	NamePlate   NamePlate
}

type PlayerResult struct {
	Profile
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

func calculateResults(splatnetDatas []*SplatnetData) LeagueResult {
	worldView := gatherWorldView(splatnetDatas)
	playerResults := gatherPlayerTimes(splatnetDatas)
	score(playerResults, &worldView)
	return LeagueResult{
		World:         worldView,
		PlayerResults: playerResults,
	}
}

func gatherWorldView(splatnetDatas []*SplatnetData) (world World) {
	sites := make(map[int]*Site)
	siteStages := make(map[int]map[int]*Stage)
	for _, splatnetData := range splatnetDatas {
		for _, splatnetSite := range splatnetData.HeroRecord.Data.HeroRecord.Sites {
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

func gatherPlayerTimes(splatnetDatas []*SplatnetData) []PlayerResult {
	playerResults := []PlayerResult{}
	for _, playerData := range splatnetDatas {
		playerResult := PlayerResult{
			Profile:            buildProfile(playerData.NsoName, playerData.NsoImageUrl, playerData.HistoryRecord),
			ExplorationRate:    playerData.HeroRecord.Data.HeroRecord.ProgressRate,
			ExplorationPhrase:  playerData.HeroRecord.Data.HeroRecord.ProgressPhrase,
			ExplorationComment: playerData.HeroRecord.Data.HeroRecord.ProgressComment,
			ResultBySite:       map[int]*SiteResult{},
		}
		for _, site := range playerData.HeroRecord.Data.HeroRecord.Sites {
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
	return playerResults
}

func buildProfile(nsoName, nsoImageURL string, history historyRecordQueryResponse) Profile {
	player := history.Data.CurrentPlayer
	badgeURLs := []string{}
	for _, badge := range player.NamePlate.Badges {
		badgeURLs = append(badgeURLs, badge.Image.URL)
	}
	return Profile{
		NsoName:     nsoName,
		NsoImageUrl: nsoImageURL,
		ByName:      player.ByName,
		Name:        player.Name,
		NameId:      player.NameId,
		NamePlate: NamePlate{
			BadgeURLs:          badgeURLs,
			TextA:              player.NamePlate.Background.TextColor.A,
			TextR:              player.NamePlate.Background.TextColor.R * 255,
			TextG:              player.NamePlate.Background.TextColor.G * 255,
			TextB:              player.NamePlate.Background.TextColor.B * 255,
			BackgroundImageURL: player.NamePlate.Background.Image.URL,
		},
	}
}

func score(playerResults []PlayerResult, worldView *World) {
	maxScore := len(playerResults)
	numStages := 0
	for _, site := range worldView.Sites {
		// score stages
		for _, stage := range site.Stages {
			numStages++
			var stageResultPointers []*StageResult
			// find stage result, if any
			for _, playerResult := range playerResults {
				if siteResult, found := playerResult.ResultBySite[site.SiteNumber]; found {
					if stageResult, found := siteResult.ResultByStage[stage.StageNumber]; found {
						stageResultPointers = append(stageResultPointers, stageResult)
					}
				}
			}
			sort.SliceStable(stageResultPointers, func(i, j int) bool { return stageResultPointers[i].Time < stageResultPointers[j].Time })
			// award ranks and points; equal times share a position and its points
			ranks := sharedRanks(len(stageResultPointers), func(i, j int) bool {
				return stageResultPointers[i].Time == stageResultPointers[j].Time
			})
			for i, stageResultPointer := range stageResultPointers {
				stageResultPointer.Rank = ranks[i]
				stageResultPointer.Score = maxScore - (ranks[i] - 1)
			}
		}
		// score site
		var siteResultPointers []*SiteResult
		// find site result, if any
		for _, playerResult := range playerResults {
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
		sort.SliceStable(siteResultPointers, func(i, j int) bool { return siteResultPointers[i].Score > siteResultPointers[j].Score })
		// award ranks; equal scores share a position
		siteRanks := sharedRanks(len(siteResultPointers), func(i, j int) bool {
			return siteResultPointers[i].Score == siteResultPointers[j].Score
		})
		for i, siteResultPointer := range siteResultPointers {
			siteResultPointer.Rank = siteRanks[i]
		}
	}
	// total score
	for i := range playerResults {
		for _, site := range worldView.Sites {
			if siteResult, found := playerResults[i].ResultBySite[site.SiteNumber]; found {
				playerResults[i].TotalScore += siteResult.Score
			}
		}
	}
	sort.SliceStable(playerResults, func(i, j int) bool { return playerResults[i].TotalScore > playerResults[j].TotalScore })
	playerRanks := sharedRanks(len(playerResults), func(i, j int) bool {
		return playerResults[i].TotalScore == playerResults[j].TotalScore
	})
	freshnessThreshold := numStages * maxScore / (len(freshness) - 1)
	for i := range playerResults {
		playerResults[i].Rank = playerRanks[i]
		playerResults[i].Freshness = freshness[playerResults[i].TotalScore/freshnessThreshold]
		playerResults[i].BestSite = 1
		playerResults[i].WorstSite = 6
		bestSiteAvgScore := -1.0
		worstSiteAvgScore := math.MaxFloat64
		for siteNumber, site := range playerResults[i].ResultBySite {
			avgScore := float64(site.Score) / float64(len(worldView.Sites[siteNumber-1].Stages))
			if avgScore > bestSiteAvgScore {
				bestSiteAvgScore = avgScore
				playerResults[i].BestSite = siteNumber
			}
			if avgScore <= worstSiteAvgScore {
				worstSiteAvgScore = avgScore
				playerResults[i].WorstSite = siteNumber
			}
		}
	}
}

// sharedRanks gives the 1-based positions of an already sorted list where
// equal entries share a position and the next one skips ahead: 1, 1, 3.
// equal reports whether neighbours i and j are tied.
func sharedRanks(n int, equal func(i, j int) bool) []int {
	ranks := make([]int, n)
	for i := range ranks {
		if i > 0 && equal(i-1, i) {
			ranks[i] = ranks[i-1]
		} else {
			ranks[i] = i + 1
		}
	}
	return ranks
}
