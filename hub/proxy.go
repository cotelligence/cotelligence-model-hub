package hub

import (
	"bytes"
	"cotelligence-model-hub/log"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const ProxyMaxAliveSeconds = 600

// Define a global HTTP client with a connection pool
var proxyClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        10,
		IdleConnTimeout:     ProxyMaxAliveSeconds * time.Second,
		DisableCompression:  true,
		MaxIdleConnsPerHost: 1,
		MaxConnsPerHost:     10,
	},
}

func ProxyRequestToPod(modelUUID, taskId string, body map[string]interface{}) (map[string]interface{}, error) {

	runPodAPI := GetRunPodAPIClient()
	// Pass the userParams to StartPrediction
	_, predictionAPIEndpoint, err := GetPredictionEndPoint(modelUUID, runPodAPI)
	if err != nil {
		return nil, err
	}

	proxyHeaders := http.Header{}
	proxyHeaders.Set("Content-Type", "application/json")

	// if stream is needed
	stream, ok := body["stream"].(bool)
	if ok && stream {
		// Set the proxy header to Prefer:respond-async
		proxyHeaders.Set("Prefer", "respond-async")
		// Add a webhook addr to the json
		body["webhook"] = "https://api-dev.cotelligence.io/cotelligence-model/webhook/" + taskId
	}

	// Marshal the predictionParams back into JSON to send as the body of the request
	jsonData, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	// Create a new request to the prediction API endpoint
	req, err := http.NewRequest("POST", predictionAPIEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header = proxyHeaders

	// Perform the request
	client := proxyClient
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	// clean up the response body
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			log.ZapLogger.Error("Failed to close response body", zap.Error(err))
		}
	}(resp.Body)

	// Read the response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// add json respbody.id = taskId
	var respBodyMap map[string]interface{}
	err = json.Unmarshal(respBody, &respBodyMap)
	if err != nil {
		return nil, err
	}
	respBodyMap["id"] = taskId
	return respBodyMap, nil
}
