package datahandling

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// nsoAccount is the minimal NSO/Coral account snapshot we attach to every
// SplatnetData payload: the user's display name and Mii avatar URL. It is
// fetched once per contestant and cached for the lifetime of an avatar (so
// most refresh cycles only pay for the SplatNet 3 dump and skip the auth
// roundtrip).
type nsoAccount struct {
	nsoName  string
	nsoImage string
}

// userResponse mirrors the shape returned by the nxapi-bridge sidecar's
// /api/user endpoint.
type userResponse struct {
	NsoName  string `json:"nsoName"`
	NsoImage string `json:"nsoImage"`
}

// recordsResponse mirrors the shape returned by /api/splatnet3/records. The
// two fields are the raw inner GraphQL payloads from `nxapi splatnet3
// dump-records` (the `result` and `player` keys of the hero/history dump files
// respectively); we wrap them back into the GraphQL envelope the app expects.
type recordsResponse struct {
	HeroResult    json.RawMessage `json:"heroResult"`
	HistoryPlayer json.RawMessage `json:"historyPlayer"`
}

func fetchNsoAccount(contestant *Contestant, sidecarURL string, client *http.Client) (*nsoAccount, error) {
	var user userResponse
	if err := getJSON(sidecarURL, "/api/user", contestant.SessionToken, client, &user); err != nil {
		return nil, err
	}
	return &nsoAccount{
		nsoName:  user.NsoName,
		nsoImage: user.NsoImage,
	}, nil
}

func fetchSplatnetData(sessionToken string, account *nsoAccount, sidecarURL string, client *http.Client) (*SplatnetData, error) {
	var records recordsResponse
	if err := getJSON(sidecarURL, "/api/splatnet3/records", sessionToken, client, &records); err != nil {
		return nil, err
	}

	heroPayload := fmt.Sprintf(`{"data":{"heroRecord":%s}}`, string(records.HeroResult))
	var heroRecord heroHistoryQueryResponse
	if err := json.Unmarshal([]byte(heroPayload), &heroRecord); err != nil {
		return nil, fmt.Errorf("unmarshal hero payload: %w", err)
	}
	historyPayload := fmt.Sprintf(`{"data":{"currentPlayer":%s}}`, string(records.HistoryPlayer))
	var historyRecord historyRecordQueryResponse
	if err := json.Unmarshal([]byte(historyPayload), &historyRecord); err != nil {
		return nil, fmt.Errorf("unmarshal history payload: %w", err)
	}
	return &SplatnetData{
		NsoName:       account.nsoName,
		NsoImageUrl:   account.nsoImage,
		HistoryRecord: historyRecord,
		HeroRecord:    heroRecord,
	}, nil
}

func getJSON(sidecarURL, path, token string, client *http.Client, out any) error {
	endpoint, err := endpointURL(sidecarURL, path, token)
	if err != nil {
		return err
	}
	resp, err := client.Get(endpoint)
	if err != nil {
		return fmt.Errorf("call nxapi-bridge %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("nxapi-bridge %s returned %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode %s response: %w", path, err)
	}
	return nil
}

// endpointURL assembles a sidecar URL. Example values for each step assume
// sidecarURL="http://nxapi:2727/", path="/api/user", token="TOKEN".
func endpointURL(sidecarURL, path, token string) (string, error) {
	base, err := url.Parse(strings.TrimRight(sidecarURL, "/")) // "http://nxapi:2727"
	if err != nil {
		return "", fmt.Errorf("invalid sidecar URL %q: %w", sidecarURL, err)
	}
	base.Path = strings.TrimRight(base.Path, "/") + path // base.Path = "/api/user"
	q := base.Query()
	q.Set("token", token)
	base.RawQuery = q.Encode() // "token=TOKEN"
	return base.String(), nil  // "http://nxapi:2727/api/user?token=TOKEN"
}
