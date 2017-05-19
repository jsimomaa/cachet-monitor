package cachet

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Sirupsen/logrus"
)

type CachetAPI struct {
	URL      string `json:"url"`
	Token    string `json:"token"`
	Insecure bool   `json:"insecure"`
}

type CachetResponse struct {
	Data json.RawMessage `json:"data"`
}

// TODO: test
func (api CachetAPI) Ping() error {
	resp, _, err := api.NewRequest("GET", "/ping", nil)
	if err != nil {
		return err
	}

	if resp.StatusCode != 200 {
		return errors.New("API Responded with non-200 status code")
	}

	return nil
}

// SendMetric adds a data point to a cachet monitor - Deprecated
func (api CachetAPI) SendMetric(id int, lag int64) {
	api.SendMetrics("lag", []int { id }, lag)
}

// SendMetrics adds a data point to a cachet monitor
func (api CachetAPI) SendMetrics(metricname string, arr []int, val int64) {
	for _, v := range arr {
		logrus.Debugf("Sending %s metric ID:%d => %v", metricname, v, val)

		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"value":     val,
			"timestamp": time.Now().Unix(),
		})

		resp, _, err := api.NewRequest("POST", "/metrics/"+strconv.Itoa(v)+"/points", jsonBytes)
		if err != nil || resp.StatusCode != 200 {
			logrus.Warnf("Could not log to metric ID: %d, err: %v", v, err)
		}
	}
}


// TODO: test
// NewRequest wraps http.NewRequest
func (api CachetAPI) NewRequest(requestType, url string, reqBody []byte) (*http.Response, CachetResponse, error) {
	req, err := http.NewRequest(requestType, api.URL+url, bytes.NewBuffer(reqBody))

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Cachet-Token", api.Token)

	transport := http.DefaultTransport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: api.Insecure}
	client := &http.Client{
		Transport: transport,
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, CachetResponse{}, err
	}

	var body struct {
		Data json.RawMessage `json:"data"`
	}
	err = json.NewDecoder(res.Body).Decode(&body)

	return res, body, err
}
