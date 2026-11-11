package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// nxapi has no Side Order support (its query IDs come from the splatnet3-types
// package, last published before the DLC), so for Side Order we only borrow a
// SplatNet 3 bullet token from `nxapi splatnet3 token --json` and send the
// GraphQL persisted queries ourselves, the same way nxapi's SplatNet3Api does.

const (
	splatnet3GraphQLURL = "https://api.lp1.av5ja.srv.nintendo.net/api/graphql"
	// SplatNet 3 web view user agent, as used by nxapi (SPLATNET3_WEBSERVICE_USERAGENT).
	splatnet3UserAgent = "Mozilla/5.0 (Linux; Android 8.0.0) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/58.0.3029.125 Mobile Safari/537.36"

	// Persisted query hashes from the SplatNet 3 web app (10.0.0, main.8f32ce85.js).
	historyRecordQuery            = "a654ecc80161a7ca5c38761c1d9e502d405eae764e2d343618b9c74b1dc0a80f"
	sideOrderRecordPaletteQuery   = "3464ece725b5f1620721d3a8415a21eeecaef71ed1a9a521199177e8f88b9984"
	sideOrderRecordChallengeQuery = "ce1ed302f8cc7c050751fa73ac2a8ae96d4795b1e8a25d27b9cea574983e837b"
	sideOrderRecordColorChipQuery = "4da51aad1d800c62b3b637b4aee16285734db5a081b0287ee6347bea611697b6"
	sideOrderRecordEnemyQuery     = "1eed33262150a80c5093892eec1ec098d41b9c67894a865da0fadaef6a2181f0"
	sideOrderChallengeDetailQuery = "79c6ed209d1725f701cf76a719e581be5f6cff76128d4f47993f85384c874393"

	tryDetailWorkers = 4 // run details fetched at once
)

var splatnet3Client = &http.Client{Timeout: 30 * time.Second}

// splatnet3Token is the output of `nxapi splatnet3 token --json`. Queries is
// nxapi's remote-config map of outdated persisted query hashes to
// [replacement hash, potentially breaking] (or null if removed).
type splatnet3Token struct {
	BulletToken string           `json:"bullet_token"`
	Version     string           `json:"version"`
	Language    string           `json:"language"`
	Queries     map[string][]any `json:"queries"`
}

// sideOrderResponse returns the raw GraphQL payloads: `data.currentPlayer`
// from HistoryRecordQuery (for the nameplate), `data.sideOrderRecord` from the
// palette, challenge, color chip and enemy queries, and `data.node` from
// SideOrderChallengeDetailQuery for each palette's fastest run and the
// overall high score run, keyed by run ID. Everything but historyPlayer is
// null/empty for players without the Side Order DLC.
type sideOrderResponse struct {
	HistoryPlayer       json.RawMessage            `json:"historyPlayer"`
	SideOrderPalettes   json.RawMessage            `json:"sideOrderPalettes"`
	SideOrderChallenge  json.RawMessage            `json:"sideOrderChallenge"`
	SideOrderColorChips json.RawMessage            `json:"sideOrderColorChips"`
	SideOrderEnemies    json.RawMessage            `json:"sideOrderEnemies"`
	TryDetails          map[string]json.RawMessage `json:"tryDetails"`
}

type sideOrderRecordData struct {
	SideOrderRecord json.RawMessage `json:"sideOrderRecord"`
}

// namedQuery is a persisted query plus where to decode its `data`.
type namedQuery struct {
	name, hash string
	out        any
}

func handleSplatnet3SideOrder(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token query parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), execTimeout)
	defer cancel()

	tok, err := fetchSplatnet3Token(ctx, token)
	if err != nil {
		log.Printf("nxapi splatnet3 token failed: %v", err)
		http.Error(w, "nxapi splatnet3 token failed", http.StatusBadGateway)
		return
	}

	var history struct {
		CurrentPlayer json.RawMessage `json:"currentPlayer"`
	}
	var palettes, challenge, colorChips, enemies sideOrderRecordData
	queries := []namedQuery{
		{"HistoryRecordQuery", historyRecordQuery, &history},
		{"SideOrderRecordPaletteQuery", sideOrderRecordPaletteQuery, &palettes},
	}
	if err := runQueries(ctx, tok, queries); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	resp := sideOrderResponse{
		HistoryPlayer:     history.CurrentPlayer,
		SideOrderPalettes: palettes.SideOrderRecord,
		TryDetails:        map[string]json.RawMessage{},
	}
	if isJSONNull(palettes.SideOrderRecord) {
		// No Side Order DLC: nothing more to fetch.
		writeJSON(w, resp)
		return
	}

	queries = []namedQuery{
		{"SideOrderRecordChallengeQuery", sideOrderRecordChallengeQuery, &challenge},
		{"SideOrderRecordColorChipQuery", sideOrderRecordColorChipQuery, &colorChips},
		{"SideOrderRecordEnemyQuery", sideOrderRecordEnemyQuery, &enemies},
	}
	if err := runQueries(ctx, tok, queries); err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	resp.SideOrderChallenge = challenge.SideOrderRecord
	resp.SideOrderColorChips = colorChips.SideOrderRecord
	resp.SideOrderEnemies = enemies.SideOrderRecord
	resp.TryDetails = fetchTryDetails(ctx, tok, tryIDs(palettes.SideOrderRecord, challenge.SideOrderRecord))
	writeJSON(w, resp)
}

// runQueries runs independent queries concurrently and returns the first
// failure.
func runQueries(ctx context.Context, tok *splatnet3Token, queries []namedQuery) error {
	errs := make([]error, len(queries))
	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := splatnet3PersistedQuery(ctx, tok, q.hash, nil, q.out); err != nil {
				log.Printf("SplatNet 3 %s failed: %v", q.name, err)
				errs[i] = fmt.Errorf("SplatNet 3 %s failed", q.name)
			}
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// tryIDs lists the runs shown in detail: each palette's fastest clear and the
// overall high score.
func tryIDs(palettes, challenge json.RawMessage) []string {
	var p struct {
		Palettes []struct {
			FastestTryResult *struct {
				ID string `json:"id"`
			} `json:"fastestTryResult"`
		} `json:"palettes"`
	}
	var c struct {
		HighestScoreTryResult *struct {
			ID string `json:"id"`
		} `json:"highestScoreTryResult"`
	}
	_ = json.Unmarshal(palettes, &p)
	_ = json.Unmarshal(challenge, &c)
	var ids []string
	for _, palette := range p.Palettes {
		if palette.FastestTryResult != nil {
			ids = append(ids, palette.FastestTryResult.ID)
		}
	}
	if c.HighestScoreTryResult != nil {
		ids = append(ids, c.HighestScoreTryResult.ID)
	}
	return ids
}

// fetchTryDetails returns SideOrderChallengeDetailQuery `data.node` per run
// ID, from the run cache when possible (see run_cache.go). Runs that fail to
// load are logged and left out so the rest of the page still renders.
func fetchTryDetails(ctx context.Context, tok *splatnet3Token, ids []string) map[string]json.RawMessage {
	details := map[string]json.RawMessage{}
	var missing []string
	for _, id := range ids {
		if data, ok := runs.get(id); ok {
			details[id] = data
		} else {
			missing = append(missing, id)
		}
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	sem := make(chan struct{}, tryDetailWorkers)
	for _, id := range missing {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			var out struct {
				Node json.RawMessage `json:"node"`
			}
			if err := splatnet3PersistedQuery(ctx, tok, sideOrderChallengeDetailQuery, map[string]any{"tryResultId": id}, &out); err != nil || isJSONNull(out.Node) {
				log.Printf("SplatNet 3 SideOrderChallengeDetailQuery failed for a run: %v", err)
				return
			}
			runs.put(id, out.Node)
			mu.Lock()
			details[id] = out.Node
			mu.Unlock()
		}()
	}
	wg.Wait()
	return details
}

// fetchSplatnet3Token gets a (cached, auto-renewed) bullet token from nxapi.
// stdout holds the token, so it is never logged; stderr is included in errors.
func fetchSplatnet3Token(ctx context.Context, sessionToken string) (*splatnet3Token, error) {
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, nxapiBin, "splatnet3", "token", "--json", "--token", sessionToken)
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w\nstderr:\n%s", err, strings.TrimSpace(stderr.String()))
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var tok splatnet3Token
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &tok); err != nil {
		return nil, fmt.Errorf("parse token output (%d bytes): %w", stdout.Len(), err)
	}
	if tok.BulletToken == "" || tok.Version == "" {
		return nil, fmt.Errorf("token output is missing bullet_token or version")
	}
	return &tok, nil
}

// splatnet3PersistedQuery runs a persisted GraphQL query and decodes its
// `data` into out.
func splatnet3PersistedQuery(ctx context.Context, tok *splatnet3Token, hash string, variables map[string]any, out any) error {
	if variables == nil {
		variables = map[string]any{}
	}
	body, err := json.Marshal(map[string]any{
		"variables": variables,
		"extensions": map[string]any{
			"persistedQuery": map[string]any{"version": 1, "sha256Hash": upgradePersistedQuery(hash, tok.Queries)},
		},
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, splatnet3GraphQLURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", splatnet3UserAgent)
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Referer", "https://api.lp1.av5ja.srv.nintendo.net/")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Authorization", "Bearer "+tok.BulletToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Web-View-Ver", tok.Version)
	req.Header.Set("Accept-Language", tok.Language)

	resp, err := splatnet3Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}

	var result struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if len(result.Errors) > 0 {
		return fmt.Errorf("GraphQL error: %s", result.Errors[0].Message)
	}
	return json.Unmarshal(result.Data, out)
}

// upgradePersistedQuery follows nxapi's remote-config query map so an outdated
// hash is swapped for its replacement, stopping at replacements flagged as
// potentially breaking (like nxapi's default "only safe" mode).
func upgradePersistedQuery(hash string, queries map[string][]any) string {
	seen := map[string]bool{hash: true}
	for {
		entry := queries[hash]
		if len(entry) != 2 {
			return hash
		}
		next, _ := entry[0].(string)
		unsafe, _ := entry[1].(bool)
		if next == "" || unsafe || seen[next] {
			return hash
		}
		seen[next] = true
		hash = next
	}
}

func isJSONNull(raw json.RawMessage) bool {
	return len(raw) == 0 || string(raw) == "null"
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
