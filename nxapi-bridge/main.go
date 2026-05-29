// nxapi-bridge is a small HTTP server that exposes a curated subset of the
// `nxapi` CLI to the alterna-freshness-league app over HTTP. It runs inside the
// nxapi sidecar so multiple app instances can share one nxapi token cache
// (`/data/persist`) without each shipping a Node.js runtime.
//
// Every endpoint shells out to the `nxapi` CLI. /api/user parses the
// human-readable output of `nxapi nso user` (which has no JSON mode) for the
// NSO/Coral Mii avatar and display name; /api/splatnet3/records runs
// `nxapi splatnet3 dump-records` and returns the resulting GraphQL payloads.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	listenAddr  = "0.0.0.0:2727"
	nxapiBin    = "nxapi"
	execTimeout = 5 * time.Minute // backstop in case nxapi hangs
)

type userResponse struct {
	NsoName  string `json:"nsoName"`
	NsoImage string `json:"nsoImage"`
}

// recordsResponse returns the raw inner GraphQL payloads from the dump files
// (the `result` and `player` keys of the hero/history dump files respectively)
// so the caller can wrap them back into whatever envelope it uses.
type recordsResponse struct {
	HeroResult    json.RawMessage `json:"heroResult"`
	HistoryPlayer json.RawMessage `json:"historyPlayer"`
}

type heroDump struct {
	Result json.RawMessage `json:"result"`
}

type historyDump struct {
	Player json.RawMessage `json:"player"`
}

// `nxapi nso user` prints two top-level objects: a "Nintendo Account" block
// (letter-icon avatar) followed by a "Nintendo Switch user" block (NSO/Coral
// Mii avatar, the thing we actually want). The output is `util.inspect`
// JS-object literal syntax, so we locate the second block by name and pull out
// the string-literal `name:` and `imageUri:` fields with a regex. If nxapi
// ever switches to JSON or restructures this output, this parser will need to
// be updated.
var (
	nxapiUserBlockStart = []byte("Nintendo Switch user {")
	nxapiNameRegex      = regexp.MustCompile(`(?m)^\s*name:\s*'((?:[^'\\]|\\.)*)'`)
	nxapiImageRegex     = regexp.MustCompile(`(?m)^\s*imageUri:\s*'((?:[^'\\]|\\.)*)'`)
)

func main() {
	http.HandleFunc("/api/user", handleUser)
	http.HandleFunc("/api/splatnet3/records", handleSplatnet3Records)
	log.Printf("nxapi-bridge listening on %s", listenAddr)
	log.Fatal(http.ListenAndServe(listenAddr, nil))
}

func handleUser(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token query parameter", http.StatusBadRequest)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, nxapiBin, "nso", "user", "--token", token)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("nxapi nso user failed: %v\noutput:\n%s", err, strings.TrimSpace(string(output)))
		http.Error(w, "nxapi nso user failed", http.StatusBadGateway)
		return
	}

	name, image, err := parseNxapiNsoUserOutput(output)
	if err != nil {
		log.Printf("failed to parse nxapi nso user output: %v\noutput:\n%s", err, strings.TrimSpace(string(output)))
		http.Error(w, "failed to parse nxapi nso user output", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(userResponse{NsoName: name, NsoImage: image})
}

func handleSplatnet3Records(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token query parameter", http.StatusBadRequest)
		return
	}

	tempDir, err := os.MkdirTemp("", "nxapi-splatnet3-*")
	if err != nil {
		http.Error(w, "failed to create temp directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.RemoveAll(tempDir)

	ctx, cancel := context.WithTimeout(r.Context(), execTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, nxapiBin, "splatnet3", "dump-records", tempDir,
		"--hero", "--history",
		"--token", token,
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("nxapi dump-records failed: %v\noutput:\n%s", err, strings.TrimSpace(string(output)))
		http.Error(w, "nxapi dump-records failed", http.StatusBadGateway)
		return
	}

	heroFile, err := newestMatchingFile(tempDir, "splatnet3-hero-*.json")
	if err != nil {
		http.Error(w, "hero file missing: "+err.Error(), http.StatusInternalServerError)
		return
	}
	historyFile, err := newestMatchingFile(tempDir, "splatnet3-history-*.json")
	if err != nil {
		http.Error(w, "history file missing: "+err.Error(), http.StatusInternalServerError)
		return
	}

	heroBytes, err := os.ReadFile(heroFile)
	if err != nil {
		http.Error(w, "failed to read hero file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	historyBytes, err := os.ReadFile(historyFile)
	if err != nil {
		http.Error(w, "failed to read history file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	var hero heroDump
	if err := json.Unmarshal(heroBytes, &hero); err != nil {
		http.Error(w, "failed to parse hero file: "+err.Error(), http.StatusInternalServerError)
		return
	}
	var history historyDump
	if err := json.Unmarshal(historyBytes, &history); err != nil {
		http.Error(w, "failed to parse history file: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(recordsResponse{
		HeroResult:    hero.Result,
		HistoryPlayer: history.Player,
	})
}

func parseNxapiNsoUserOutput(output []byte) (name, image string, err error) {
	idx := strings.Index(string(output), string(nxapiUserBlockStart))
	if idx == -1 {
		return "", "", fmt.Errorf("'Nintendo Switch user' block not found")
	}
	block := output[idx:]

	// Walk braces to bound the block and avoid picking up fields from later objects.
	depth := 0
	end := -1
	for i, b := range block {
		switch b {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end = i + 1
			}
		}
		if end != -1 {
			break
		}
	}
	if end == -1 {
		return "", "", fmt.Errorf("malformed 'Nintendo Switch user' block (no closing brace)")
	}
	block = block[:end]

	nameMatch := nxapiNameRegex.FindSubmatch(block)
	if nameMatch == nil {
		return "", "", fmt.Errorf("name field not found in 'Nintendo Switch user' block")
	}
	imageMatch := nxapiImageRegex.FindSubmatch(block)
	if imageMatch == nil {
		return "", "", fmt.Errorf("imageUri field not found in 'Nintendo Switch user' block")
	}
	return unescapeJSStringLiteral(string(nameMatch[1])), unescapeJSStringLiteral(string(imageMatch[1])), nil
}

// util.inspect renders inner single quotes as `\'` and backslashes as `\\`;
// it also emits the usual `\n`, `\r`, `\t`, and `\xNN`/`\uNNNN` escapes for
// non-printables. Splatoon-style display names rarely include any of those, so
// we only undo the common ones. Anything fancier will surface as a slightly
// wrong name rather than a parse failure.
func unescapeJSStringLiteral(s string) string {
	s = strings.ReplaceAll(s, `\'`, `'`)
	s = strings.ReplaceAll(s, `\\`, `\`)
	return s
}

func newestMatchingFile(directory, pattern string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(directory, pattern))
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no files matched %s", pattern)
	}
	sort.Strings(paths)
	return paths[len(paths)-1], nil
}
