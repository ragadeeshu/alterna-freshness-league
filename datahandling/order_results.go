package datahandling

import (
	"encoding/base64"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	// The palette whose comparison is worth double points.
	doublePointsPaletteKey = "Agent8"
	// The chip board is 9×4 slots.
	sideOrderChipSlots = 36
)

// SplatNet 3 returns the Side Order palette weapons with their Japanese names.
var sideOrderWeaponNames = map[string]string{
	"シューター":    "Order Shot",
	"ローラー":     "Order Roller",
	"チャージャー":   "Order Charger",
	"マニューバー":   "Order Dualies",
	"シェルター":    "Order Brella",
	"ストリンガー":   "Order Stringer",
	"ワイパー":     "Order Splatana",
	"スロッシャー":   "Order Slosher",
	"ブラスター":    "Order Blaster",
	"フデ":       "Order Brush",
	"スピナー":     "Order Spinner",
	"オクタシューター": "Octo Shot",
}

type OrderLeagueResult struct {
	PlayerResults []*OrderPlayerResult // sorted by total score
	Palettes      []*OrderPalette      // SplatNet order (Pearl … Eight)
	HighScores    []*OrderRun          // each player's highest-scoring run, in standings order
}

type OrderPlayerResult struct {
	Profile
	TotalScore      int
	Rank            int
	PalettesCleared int
	PaletteCount    int

	BestPalette, WorstPalette string  // own fastest / slowest clear
	BestTime, WorstTime       float64 // seconds
	MostClearsPalette         string
	MostClears                int
	TotalClears               int
	ChipsMaxed, ChipsTotal    int // color chips taken to their max level, out of all chips
	EnemiesSplatted           int
	ChipUsage                 []OrderChipUsage // color chip usage by color, most used first
}

type OrderChipUsage struct {
	Group   string // SplatNet's chip color group in lower case: power, support, range, move, lucky, drone
	Name    string
	IconURL string
	Rate    float64 // 0–1
}

type OrderPalette struct {
	ID          string
	Key         string // e.g. "Hime", decoded from the SplatNet palette ID
	Name        string
	NpcImageURL string
	Multiplier  int
	Runs        []*OrderRun // each player's fastest clear, in standings order
}

// OrderRun is one player's run in one comparison, as shown on a run card.
type OrderRun struct {
	Player        *OrderPlayerResult
	Placement     int    // league placing in this comparison, 0 if not placed
	Points        int    // league points
	Highlight     string // where the winner's splash goes: "time", "score" or ""
	NotCleared    bool   // the palette isn't cleared yet; only Floor (highest reached) is known
	ShowPalette   bool   // name the palette on the card instead of the player
	PaletteClears int    // the player's total clears of this run's palette

	Floor          int
	Result         string  // "Clear! 28:26.20", "Defeated", "Gave up", "Not cleared yet"
	Time           float64 // clear time in seconds
	Score          int
	HasDetail      bool // false if the run details couldn't be loaded
	PlayedDate     string
	Hacks          int
	Coin           int
	PaletteKey     string
	PaletteName    string
	PaletteIconURL string
	IsHachi        bool
	Weapons        SideOrderWeapons
	KeyStatuses    []int    // 2 unlocked, 1 unlocking, 0 locked
	DroneSlots     []string // image URL per drone action slot, "" if empty
	ChipSlots      []OrderChipSlot
	Detail         *OrderRunDetail // the click-through summary; nil without run details
}

// OrderRunDetail is SplatNet's palette (color chip) summary for a run plus
// Marina's hacks and the run footer.
type OrderRunDetail struct {
	ChipGroups     []OrderChipGroup // colors with chips, in SplatNet order
	HackTotal      int
	HackCategories []OrderHackCategory
	Bosses         []SideOrderNamedImage
	MembuxSpent    int
	FloorResets    int
}

type OrderChipGroup struct {
	Group string // power, support, range, move, lucky, drone
	Title string
	Chips []OrderRunChip
}

type OrderRunChip struct {
	Name         string
	ChipImageURL string
	IconURL      string
	Level        int
	MaxLevel     int
	LevelDots    []bool // one per level, true up to Level
}

type OrderHackCategory struct {
	Name  string
	Level string // "--" when the category isn't available
	Hacks []OrderHack
}

type OrderHack struct {
	Name    string
	IconURL string
	Level   int
	Meter   []bool // maxLevel+1 cells (the first is level 0), true up to Level
}

type SideOrderNamedImage struct {
	Name, ImageURL string
}

type OrderChipSlot struct {
	Name     string
	ImageURL string
	State    string // "set", "empty" or "disabled"
}

type SideOrderWeapons struct {
	MainName, MainImageURL       string
	SubName, SubImageURL         string
	SpecialName, SpecialImageURL string
}

func calculateOrderResults(datas []*SideOrderData) OrderLeagueResult {
	var result OrderLeagueResult
	palettesByID := map[string]*OrderPalette{}
	var playerDatas []*SideOrderData
	for _, data := range datas {
		// Players without the DLC aren't part of the Side Order league.
		if data.Palettes == nil {
			continue
		}
		playerDatas = append(playerDatas, data)
		result.PlayerResults = append(result.PlayerResults, &OrderPlayerResult{
			Profile:      buildProfile(data.NsoName, data.NsoImageUrl, data.HistoryRecord),
			PaletteCount: len(data.Palettes.Palettes),
		})
	}
	numPlayers := len(result.PlayerResults)

	for i, data := range playerDatas {
		player := result.PlayerResults[i]
		clearsByPalette := map[string]int{}
		// Palette list in SplatNet order, plus every player's run on each one.
		for _, record := range data.Palettes.Palettes {
			palette, found := palettesByID[record.Palette.ID]
			if !found {
				key := paletteKey(record.Palette.ID)
				palette = &OrderPalette{
					ID:          record.Palette.ID,
					Key:         key,
					Name:        record.Palette.Name,
					NpcImageURL: record.Palette.Image.URL,
					Multiplier:  1,
				}
				if key == doublePointsPaletteKey {
					palette.Multiplier = 2
				}
				palettesByID[record.Palette.ID] = palette
				result.Palettes = append(result.Palettes, palette)
			}
			clearsByPalette[record.Palette.ID] = record.ClearCount
			player.TotalClears += record.ClearCount
			if record.ClearCount > player.MostClears {
				player.MostClearsPalette, player.MostClears = palette.Name, record.ClearCount
			}

			var run *OrderRun
			if try := record.FastestTryResult; try != nil {
				run = buildRun(try, data.TryDetails[try.ID], palettesByID)
				player.PalettesCleared++
				if player.BestPalette == "" || try.ClearTimeSec < player.BestTime {
					player.BestPalette, player.BestTime = palette.Name, try.ClearTimeSec
				}
				if player.WorstPalette == "" || try.ClearTimeSec > player.WorstTime {
					player.WorstPalette, player.WorstTime = palette.Name, try.ClearTimeSec
				}
			} else {
				run = &OrderRun{
					NotCleared:     true,
					Floor:          record.HighestFloor,
					Result:         "Not cleared yet",
					PaletteKey:     palette.Key,
					PaletteName:    palette.Name,
					PaletteIconURL: palette.NpcImageURL,
					IsHachi:        palette.Key == doublePointsPaletteKey,
				}
			}
			run.Player = player
			run.PaletteClears = record.ClearCount
			palette.Runs = append(palette.Runs, run)
		}
		addPlayerStats(player, data)

		if data.Challenge != nil && data.Challenge.HighestScoreTryResult != nil {
			try := data.Challenge.HighestScoreTryResult
			run := buildRun(try, data.TryDetails[try.ID], palettesByID)
			run.Player = player
			run.ShowPalette = true
			run.PaletteClears = clearsByPalette[try.Palette.ID]
			result.HighScores = append(result.HighScores, run)
		}
	}

	// Score palettes: fastest clear wins.
	for _, palette := range result.Palettes {
		var cleared []*OrderRun
		for _, run := range palette.Runs {
			if !run.NotCleared {
				cleared = append(cleared, run)
			}
		}
		sort.SliceStable(cleared, func(i, j int) bool { return cleared[i].Time < cleared[j].Time })
		// Equal times share a placing and its points.
		ranks := sharedRanks(len(cleared), func(i, j int) bool { return cleared[i].Time == cleared[j].Time })
		for i, run := range cleared {
			run.Placement = ranks[i]
			run.Points = palette.Multiplier * (numPlayers - (ranks[i] - 1))
			run.Player.TotalScore += run.Points
			if run.Placement == 1 {
				run.Highlight = "time"
			}
		}
	}

	// Score the overall high score: largest score wins.
	byScore := append([]*OrderRun{}, result.HighScores...)
	sort.SliceStable(byScore, func(i, j int) bool { return byScore[i].Score > byScore[j].Score })
	scoreRanks := sharedRanks(len(byScore), func(i, j int) bool { return byScore[i].Score == byScore[j].Score })
	for i, run := range byScore {
		run.Placement = scoreRanks[i]
		run.Points = numPlayers - (scoreRanks[i] - 1)
		run.Player.TotalScore += run.Points
		if run.Placement == 1 {
			run.Highlight = "score"
		}
	}

	// Standings, then show every comparison in standings order.
	sort.SliceStable(result.PlayerResults, func(i, j int) bool {
		return result.PlayerResults[i].TotalScore > result.PlayerResults[j].TotalScore
	})
	playerRanks := sharedRanks(len(result.PlayerResults), func(i, j int) bool {
		return result.PlayerResults[i].TotalScore == result.PlayerResults[j].TotalScore
	})
	for i, player := range result.PlayerResults {
		player.Rank = playerRanks[i]
	}
	byStandings := func(runs []*OrderRun) {
		sort.SliceStable(runs, func(i, j int) bool { return runs[i].Player.Rank < runs[j].Player.Rank })
	}
	for _, palette := range result.Palettes {
		byStandings(palette.Runs)
	}
	byStandings(result.HighScores)
	return result
}

// addPlayerStats fills in the color chip and enemy stats shown in the standings.
func addPlayerStats(player *OrderPlayerResult, data *SideOrderData) {
	if data.ColorChips != nil {
		for _, list := range data.ColorChips.lists() {
			for _, node := range list.Nodes {
				player.ChipsTotal++
				if node.HighestAccumulateCount >= node.Chip.MaxAccumulateCount {
					player.ChipsMaxed++
				}
			}
		}
	}
	if data.Enemies != nil {
		for _, record := range data.Enemies.DefeatEnemyRecords {
			player.EnemiesSplatted += record.DefeatCount
		}
	}
	if data.Challenge != nil {
		for _, rate := range data.Challenge.ChipUsageRates {
			player.ChipUsage = append(player.ChipUsage, OrderChipUsage{
				Group:   strings.ToLower(rate.ChipCategoryGroup.Group),
				Name:    rate.ChipCategoryGroup.Name,
				IconURL: rate.ChipCategoryGroup.Image.URL,
				Rate:    rate.UsageRate,
			})
		}
		sort.SliceStable(player.ChipUsage, func(i, j int) bool { return player.ChipUsage[i].Rate > player.ChipUsage[j].Rate })
	}
}

// buildRun prepares a run card from the run summary, enriched with the run
// details when they could be loaded.
func buildRun(try *sideOrderTryResult, detail *sideOrderTryDetail, palettesByID map[string]*OrderPalette) *OrderRun {
	run := &OrderRun{
		Floor:   try.Floor,
		Result:  sideOrderResultText(try),
		Time:    try.ClearTimeSec,
		Score:   try.Score,
		Weapons: sideOrderWeapons(try),
	}
	if palette, found := palettesByID[try.Palette.ID]; found {
		run.PaletteKey, run.PaletteName, run.PaletteIconURL = palette.Key, palette.Name, palette.NpcImageURL
		run.IsHachi = palette.Key == doublePointsPaletteKey
	}
	if detail == nil {
		return run
	}
	run.HasDetail = true
	run.PlayedDate = formatPlayedDate(detail.PlayedTime)
	run.Hacks = detail.HackTotalLevel
	run.Coin = detail.Coin
	run.KeyStatuses = detail.PaletteKeyStatus
	for _, slot := range detail.Slots {
		if slot.IsEmpty {
			run.DroneSlots = append(run.DroneSlots, "")
		} else {
			run.DroneSlots = append(run.DroneSlots, slot.Image.URL)
		}
	}
	for i := 0; i < sideOrderChipSlots; i++ {
		switch {
		case i < len(detail.Chips.Nodes):
			chip := detail.Chips.Nodes[i]
			run.ChipSlots = append(run.ChipSlots, OrderChipSlot{Name: chip.Name, ImageURL: chip.ChipCategory.Image.URL, State: "set"})
		case i < detail.AvailableChipSlotCount:
			run.ChipSlots = append(run.ChipSlots, OrderChipSlot{State: "empty"})
		default:
			run.ChipSlots = append(run.ChipSlots, OrderChipSlot{State: "disabled"})
		}
	}
	run.Detail = buildRunDetail(detail)
	return run
}

func buildRunDetail(detail *sideOrderTryDetail) *OrderRunDetail {
	result := &OrderRunDetail{
		HackTotal:   detail.HackTotalLevel,
		MembuxSpent: detail.PaidCoin,
		FloorResets: detail.Retry,
	}
	for _, group := range []struct {
		key, title string
		list       sideOrderRunChipList
	}{
		{"power", "Power", detail.PowerColorChips},
		{"support", "Support", detail.SupportColorChips},
		{"range", "Range", detail.RangeColorChips},
		{"move", "Mobility", detail.MoveColorChips},
		{"lucky", "Lucky", detail.LuckyColorChips},
		{"drone", "Drone", detail.DroneColorChips},
	} {
		if len(group.list.Nodes) == 0 {
			continue
		}
		chipGroup := OrderChipGroup{Group: group.key, Title: group.title}
		for _, node := range group.list.Nodes {
			chip := OrderRunChip{
				Name:         node.Chip.Name,
				ChipImageURL: node.Chip.ChipCategory.Image.URL,
				IconURL:      node.Chip.Image.URL,
				Level:        node.CurrentAccumulateCount,
				MaxLevel:     node.Chip.MaxAccumulateCount,
			}
			for i := 0; i < chip.MaxLevel; i++ {
				chip.LevelDots = append(chip.LevelDots, i < chip.Level)
			}
			chipGroup.Chips = append(chipGroup.Chips, chip)
		}
		result.ChipGroups = append(result.ChipGroups, chipGroup)
	}
	for _, category := range []struct {
		level *int
		list  sideOrderHackList
	}{
		{detail.PlayerHackLevel, detail.PlayerHackSettings},
		{detail.RandomHackLevel, detail.RandomHackSettings},
		{detail.DroneHackLevel, detail.DroneHackSettings},
		{detail.BossHackLevel, detail.BossHackSettings},
		{detail.ShopHackLevel, detail.ShopHackSettings},
		{detail.IkuraHackLevel, detail.IkuraHackSettings},
		{detail.GemHackLevel, detail.GemHackSettings},
	} {
		if len(category.list.Nodes) == 0 {
			continue
		}
		hackCategory := OrderHackCategory{Name: category.list.Nodes[0].Hack.HackCategory.Name, Level: "--"}
		if category.level != nil {
			hackCategory.Level = fmt.Sprint(*category.level)
		}
		for _, node := range category.list.Nodes {
			hack := OrderHack{Name: node.Hack.Name, IconURL: node.Hack.Image.URL, Level: node.CurrentLevel}
			for i := 0; i <= node.Hack.MaxLevel; i++ {
				hack.Meter = append(hack.Meter, i <= node.CurrentLevel)
			}
			hackCategory.Hacks = append(hackCategory.Hacks, hack)
		}
		result.HackCategories = append(result.HackCategories, hackCategory)
	}
	for _, boss := range []sideOrderNamedImage{detail.FirstBoss, detail.SecondBoss} {
		if boss.Name != "" {
			result.Bosses = append(result.Bosses, SideOrderNamedImage{Name: boss.Name, ImageURL: boss.Image.URL})
		}
	}
	return result
}

// paletteKey turns a SplatNet palette ID (base64 "SideOrderPalette-Hime")
// into its internal name ("Hime").
func paletteKey(id string) string {
	decoded, err := base64.StdEncoding.DecodeString(id)
	if err != nil {
		return id
	}
	return strings.TrimPrefix(string(decoded), "SideOrderPalette-")
}

func sideOrderWeapons(try *sideOrderTryResult) SideOrderWeapons {
	mainName := try.Palette.Weapon.Name
	if english, found := sideOrderWeaponNames[mainName]; found {
		mainName = english
	}
	return SideOrderWeapons{
		MainName:        mainName,
		MainImageURL:    try.Palette.Weapon.Image.URL,
		SubName:         try.SubWeapon.Name,
		SubImageURL:     try.SubWeapon.Image.URL,
		SpecialName:     try.SpecialWeapon.Name,
		SpecialImageURL: try.SpecialWeapon.Image.URL,
	}
}

func sideOrderResultText(try *sideOrderTryResult) string {
	switch try.Result {
	case sideOrderResultClear:
		return "Clear! " + FormatTime(try.ClearTimeSec)
	case sideOrderResultDefeated:
		return "Defeated"
	case sideOrderResultGaveUp:
		return "Gave up"
	}
	return ""
}

func formatPlayedDate(playedTime string) string {
	t, err := time.Parse(time.RFC3339, playedTime)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// FormatTime renders seconds as mm:ss.cc, as SplatNet does.
func FormatTime(seconds float64) string {
	return fmt.Sprintf("%02d:%05.2f", int(seconds/60), math.Mod(seconds, 60))
}
