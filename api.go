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
func (api CachetAPI) SendMetric(l *logrus.Entry, id int, lag int64) {
	api.SendMetrics(l, "lag", []int { id }, lag)
}

// SendMetrics adds a data point to a cachet monitor
func (api CachetAPI) SendMetrics(l *logrus.Entry, metricname string, arr []int, val int64) {
	for _, v := range arr {
		l.Infof("Sending %s metric ID:%d => %v", metricname, v, val)

		jsonBytes, _ := json.Marshal(map[string]interface{}{
			"value":     val,
			"timestamp": time.Now().Unix(),
		})

		resp,_,_ := api.NewRequest("POST", "/metrics/"+strconv.Itoa(v)+"/points", jsonBytes)

		if resp != nil {
			if resp.StatusCode == 200 {
				l.Debugf("Sending %s metric ID:%d => %v, returns %d", metricname, v, val, resp.StatusCode)
			} else {
				l.Warnf("Sending %s metric ID:%d => %v, returns %d", metricname, v, val, resp.StatusCode)
			}
		} else {
			l.Warnf("Sending %s metric ID:%d => %v, no return", metricname, v, val)
		}
	}
}

// TODO: test
// GetComponentData
func (api CachetAPI) GetComponentData(compid int) (Component) {
	logrus.Debugf("Getting data from component ID:%d", compid)

	resp, body, err := api.NewRequest("GET", "/components/"+strconv.Itoa(compid), []byte(""))

	if err != nil || resp.StatusCode != 200 {
		logrus.Warnf("Could not get data from component (id: %d, status: %d, err: %v)", compid, resp.StatusCode, err)
	}

	var compInfo Component

	err = json.Unmarshal(body.Data, &compInfo)

	return compInfo
}

// SetComponentStatus
func (api CachetAPI) SetComponentStatus(comp *AbstractMonitor, status int) (Component) {
	logrus.Debugf("Setting new status (%d) to component ID: %d (instead of %d)", status, comp.ComponentID, comp.currentStatus)

	jsonBytes, _ := json.Marshal(map[string]interface{}{
		"status":     status,
	})

	resp, body, err := api.NewRequest("PUT", "/components/"+strconv.Itoa(comp.ComponentID), jsonBytes)

	if err != nil || resp.StatusCode != 200 {
		logrus.Warnf("Could not get data from component (id: %d, status: %d, err: %v)", comp.ComponentID, resp.StatusCode, err)
	}
	comp.currentStatus = status

	var compInfo Component

	err = json.Unmarshal(body.Data, &compInfo)

	return compInfo
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
