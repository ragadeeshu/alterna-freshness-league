package datahandling

// Side Order payloads from the nxapi-bridge /api/splatnet3/side-order endpoint
// (SplatNet 3's SideOrderRecord* queries and SideOrderChallengeDetailQuery).
// Only the fields the /order page uses are decoded.

type sideOrderImage struct {
	URL string `json:"url"`
}

type sideOrderNamedImage struct {
	Name  string         `json:"name"`
	Image sideOrderImage `json:"image"`
}

// Try result codes as returned by SplatNet 3.
const (
	sideOrderResultClear    = 1
	sideOrderResultDefeated = 2
	sideOrderResultGaveUp   = 3
)

// sideOrderTryResult is the run summary used in record lists.
type sideOrderTryResult struct {
	ID           string  `json:"id"`
	Floor        int     `json:"floor"`
	Result       int     `json:"result"`
	ClearTimeSec float64 `json:"clearTimeSec"` // null (0) unless Result is a clear
	Score        int     `json:"score"`
	Palette      struct {
		ID     string              `json:"id"`
		Weapon sideOrderNamedImage `json:"weapon"`
	} `json:"palette"`
	SubWeapon     sideOrderNamedImage `json:"subWeapon"`
	SpecialWeapon sideOrderNamedImage `json:"specialWeapon"`
}

// sideOrderTryDetail is one run from SideOrderChallengeDetailQuery.
type sideOrderTryDetail struct {
	sideOrderTryResult
	PlayedTime             string `json:"playedTime"`
	Coin                   int    `json:"coin"`
	HackTotalLevel         int    `json:"hackTotalLevel"`
	AvailableChipSlotCount int    `json:"availableChipSlotCount"`
	PaletteKeyStatus       []int  `json:"paletteKeyStatus"`
	Slots                  []struct {
		IsEmpty bool           `json:"isEmpty"`
		Image   sideOrderImage `json:"image"`
	} `json:"slots"`
	Chips struct {
		Nodes []struct {
			Name         string `json:"name"`
			ChipCategory struct {
				Image sideOrderImage `json:"image"`
			} `json:"chipCategory"`
		} `json:"nodes"`
	} `json:"chips"`

	// Color chips collected in the run, per color, with their levels.
	PowerColorChips   sideOrderRunChipList `json:"powerColorChips"`
	SupportColorChips sideOrderRunChipList `json:"supportColorChips"`
	RangeColorChips   sideOrderRunChipList `json:"rangeColorChips"`
	MoveColorChips    sideOrderRunChipList `json:"moveColorChips"`
	LuckyColorChips   sideOrderRunChipList `json:"luckyColorChips"`
	DroneColorChips   sideOrderRunChipList `json:"droneColorChips"`

	// Marina's hacks, per category. A category level is null when the
	// category isn't available (shown as "--" in SplatNet).
	PlayerHackLevel    *int              `json:"playerHackLevel"`
	PlayerHackSettings sideOrderHackList `json:"playerHackSettings"`
	RandomHackLevel    *int              `json:"randomHackLevel"`
	RandomHackSettings sideOrderHackList `json:"randomHackSettings"`
	DroneHackLevel     *int              `json:"droneHackLevel"`
	DroneHackSettings  sideOrderHackList `json:"droneHackSettings"`
	BossHackLevel      *int              `json:"bossHackLevel"`
	BossHackSettings   sideOrderHackList `json:"bossHackSettings"`
	ShopHackLevel      *int              `json:"shopHackLevel"`
	ShopHackSettings   sideOrderHackList `json:"shopHackSettings"`
	IkuraHackLevel     *int              `json:"ikuraHackLevel"`
	IkuraHackSettings  sideOrderHackList `json:"ikuraHackSettings"`
	GemHackLevel       *int              `json:"gemHackLevel"`
	GemHackSettings    sideOrderHackList `json:"gemHackSettings"`

	FirstBoss  sideOrderNamedImage `json:"firstBoss"`
	SecondBoss sideOrderNamedImage `json:"secondBoss"`
	PaidCoin   int                 `json:"paidCoin"` // membux spent at vending machines
	Retry      int                 `json:"retry"`    // floor resets
}

type sideOrderRunChipList struct {
	Nodes []struct {
		Chip struct {
			Name               string         `json:"name"`
			Image              sideOrderImage `json:"image"` // effect icon
			MaxAccumulateCount int            `json:"maxAccumulateCount"`
			ChipCategory       struct {
				Image sideOrderImage `json:"image"` // color chip
			} `json:"chipCategory"`
		} `json:"chip"`
		CurrentAccumulateCount int `json:"currentAccumulateCount"`
	} `json:"nodes"`
}

type sideOrderHackList struct {
	Nodes []struct {
		Hack struct {
			Name         string         `json:"name"`
			Image        sideOrderImage `json:"image"`
			MaxLevel     int            `json:"maxLevel"`
			HackCategory struct {
				Name string `json:"name"`
			} `json:"hackCategory"`
		} `json:"hack"`
		CurrentLevel int `json:"currentLevel"`
	} `json:"nodes"`
}

type sideOrderPaletteRecord struct {
	Palettes []struct {
		Palette struct {
			ID    string         `json:"id"`
			Name  string         `json:"name"`
			Image sideOrderImage `json:"image"`
		} `json:"palette"`
		ClearCount       int                 `json:"clearCount"`
		HighestFloor     int                 `json:"highestFloor"`
		FastestTryResult *sideOrderTryResult `json:"fastestTryResult"` // nil until the palette is cleared
	} `json:"palettes"`
}

type sideOrderChallengeRecord struct {
	HighestScoreTryResult *sideOrderTryResult `json:"highestScoreTryResult"`
	ChipUsageRates        []struct {
		ChipCategoryGroup struct {
			Group string         `json:"group"` // POWER, SUPPORT, RANGE, MOVE, LUCKY, DRONE
			Name  string         `json:"name"`
			Image sideOrderImage `json:"image"`
		} `json:"chipCategoryGroup"`
		UsageRate float64 `json:"usageRate"`
	} `json:"chipUsageRates"`
}

type sideOrderColorChipList struct {
	Nodes []struct {
		Chip struct {
			MaxAccumulateCount int `json:"maxAccumulateCount"`
		} `json:"chip"`
		HighestAccumulateCount int `json:"highestAccumulateCount"`
	} `json:"nodes"`
}

type sideOrderColorChipRecord struct {
	Power   sideOrderColorChipList `json:"powerColorChips"`
	Support sideOrderColorChipList `json:"supportColorChips"`
	Range   sideOrderColorChipList `json:"rangeColorChips"`
	Move    sideOrderColorChipList `json:"moveColorChips"`
	Lucky   sideOrderColorChipList `json:"luckyColorChips"`
	Drone   sideOrderColorChipList `json:"droneColorChips"`
}

func (r *sideOrderColorChipRecord) lists() []sideOrderColorChipList {
	return []sideOrderColorChipList{r.Power, r.Support, r.Range, r.Move, r.Lucky, r.Drone}
}

type sideOrderEnemyRecord struct {
	DefeatEnemyRecords []struct {
		DefeatCount int `json:"defeatCount"`
	} `json:"defeatEnemyRecords"`
}

// SideOrderData is one contestant's Side Order snapshot. Everything but the
// profile is nil for players without the Side Order DLC. It is also the JSON
// shape served by proxy instances on /order.
type SideOrderData struct {
	NsoName       string                         `json:"nsoName"`
	NsoImageUrl   string                         `json:"nsoImageUrl"`
	HistoryRecord historyRecordQueryResponse     `json:"historyRecord"`
	Palettes      *sideOrderPaletteRecord        `json:"sideOrderPalettes"`
	Challenge     *sideOrderChallengeRecord      `json:"sideOrderChallenge"`
	ColorChips    *sideOrderColorChipRecord      `json:"sideOrderColorChips"`
	Enemies       *sideOrderEnemyRecord          `json:"sideOrderEnemies"`
	TryDetails    map[string]*sideOrderTryDetail `json:"tryDetails"` // by run ID
}
