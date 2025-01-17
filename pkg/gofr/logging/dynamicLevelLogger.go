package logging

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"time"

	"gofr.dev/pkg/gofr/service"
)

const (
	requestTimeout = 5 * time.Second
)

func NewRemoteLogger(level Level, remoteConfigURL, loggerFetchInterval string) Logger {
	interval, err := strconv.Atoi(loggerFetchInterval)
	if err != nil {
		interval = 15
	}

	l := remoteLogger{
		remoteURL:          remoteConfigURL,
		Logger:             NewLogger(level),
		levelFetchInterval: interval,
		currentLevel:       level,
	}

	if remoteConfigURL != "" {
		go l.UpdateLogLevel()
	}

	return l
}

type remoteLogger struct {
	remoteURL          string
	levelFetchInterval int
	currentLevel       Level
	Logger
}

func (r *remoteLogger) UpdateLogLevel() {
	interval := time.Duration(r.levelFetchInterval) * time.Second
	ticker := time.NewTicker(interval)

	defer ticker.Stop()

	remoteService := service.NewHTTPService(r.remoteURL, r.Logger, nil)

	for range ticker.C {
		newLevel, err := fetchAndUpdateLogLevel(remoteService, r.currentLevel)
		if err == nil {
			r.changeLevel(newLevel)

			if r.currentLevel != newLevel {
				r.Infof("LOG_LEVEL updated from %v to %v", r.currentLevel, newLevel)
				r.currentLevel = newLevel
			}
		}
	}
}

func fetchAndUpdateLogLevel(remoteService service.HTTP, currentLevel Level) (Level, error) {
	ctx, cancel := context.WithTimeout(context.Background(), requestTimeout) // Set timeout for 5 seconds
	defer cancel()

	resp, err := remoteService.Get(ctx, "", nil)
	if err != nil {
		return currentLevel, err
	}
	defer resp.Body.Close()

	var response struct {
		Data []struct {
			ServiceName string            `json:"serviceName"`
			Level       map[string]string `json:"logLevel"`
		} `json:"data"`
	}

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return currentLevel, err
	}

	err = json.Unmarshal(responseBody, &response)
	if err != nil {
		return currentLevel, err
	}

	if len(response.Data) > 0 {
		newLevel := GetLevelFromString(response.Data[0].Level["LOG_LEVEL"])
		return newLevel, nil
	}

	return currentLevel, nil
}
