package chain

import (
	"cotelligence-model-hub/config"
	"cotelligence-model-hub/log"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

var (
	MaxPodsCnt = 1
	url        = config.GetConfig().CotelligenceRwaEndpoint
)

type Data struct {
	ID int `json:"id"`
}

func UpdateMaxPodsCnt() {
	ticker := time.NewTicker(10 * time.Second)
	go func() {
		for range ticker.C {
			resp, err := http.Get(url)
			if err != nil {
				// handle error
				continue
			}
			defer resp.Body.Close()

			var data []Data
			err = json.NewDecoder(resp.Body).Decode(&data)
			if err != nil {
				// handle error
				log.ZapLogger.Error("Failed to decode response", zap.Error(err))
				continue
			}
			MaxPodsCnt = len(data)
		}
	}()
}

func init() {
	UpdateMaxPodsCnt()
}
