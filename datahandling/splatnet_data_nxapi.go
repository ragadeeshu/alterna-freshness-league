package datahandling

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

type nxapiHeroDump struct {
	Result json.RawMessage `json:"result"`
}

type nxapiHistoryDump struct {
	Player json.RawMessage `json:"player"`
}

func newestMatchingFile(directory string, pattern string) (string, error) {
	paths, err := filepath.Glob(filepath.Join(directory, pattern))
	if err != nil {
		return "", fmt.Errorf("glob failed: %w", err)
	}
	if len(paths) == 0 {
		return "", fmt.Errorf("no files found for %s", pattern)
	}
	sort.Strings(paths)
	return paths[len(paths)-1], nil
}

func parseHeroRecord(path string) (*heroHistoryQueryResponse, error) {
	byteValue, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read hero file: %w", err)
	}
	var dump nxapiHeroDump
	if err := json.Unmarshal(byteValue, &dump); err != nil {
		return nil, fmt.Errorf("failed to unmarshal hero dump: %w", err)
	}

	heroPayload := fmt.Sprintf(`{"data":{"heroRecord":%s}}`, string(dump.Result))
	var heroRecord heroHistoryQueryResponse
	if err := json.Unmarshal([]byte(heroPayload), &heroRecord); err != nil {
		return nil, fmt.Errorf("failed to map hero dump to app schema: %w", err)
	}
	return &heroRecord, nil
}

func parseHistoryRecord(path string) (*historyRecordQueryResponse, error) {
	byteValue, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read history file: %w", err)
	}
	var dump nxapiHistoryDump
	if err := json.Unmarshal(byteValue, &dump); err != nil {
		return nil, fmt.Errorf("failed to unmarshal history dump: %w", err)
	}

	historyPayload := fmt.Sprintf(`{"data":{"currentPlayer":%s}}`, string(dump.Player))
	var historyRecord historyRecordQueryResponse
	if err := json.Unmarshal([]byte(historyPayload), &historyRecord); err != nil {
		return nil, fmt.Errorf("failed to map history dump to app schema: %w", err)
	}
	return &historyRecord, nil
}
