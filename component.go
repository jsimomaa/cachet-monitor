package cachet

import (
	"encoding/json"
	"strconv"

	"github.com/Sirupsen/logrus"
)

// Component Cachet data model
type Component struct {
	ID     int `json:"id"`
	Name   string `json:"name"`
	Status int `json:"status"`
	Enabled bool `json:"enabled"`
}

// LoadCurrentIncident - Returns current incident
func (comp *Component) LoadCurrentIncident(cfg *CachetMonitor) (*Incident, error) {
	jsonBytes, _ := json.Marshal(map[string]interface{}{
		"component_id":	strconv.Itoa(comp.ID),
		"status":	1,
		"per_page":	1,
	})

	resp, body, err := cfg.API.NewRequest("GET", "/incidents", jsonBytes)

	if err != nil || resp.StatusCode != 200 {
		logrus.Warnf("Could not get data from component (id: %d, status: %d, err: %v)", comp.ID, resp.StatusCode, err)
		return nil, err
	}

	incidentInfoA := []Incident{}

	if e := json.Unmarshal(body.Data, &incidentInfoA); e != nil {
	        logrus.Warnf("Error decoding JSON: %v\n", e)
	}

	if len(incidentInfoA) == 0 {
		return nil, err
	}

	return &incidentInfoA[0], err
}