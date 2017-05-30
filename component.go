package cachet

import (
	"encoding/json"
	"strconv"
	"fmt"
)

// Component Cachet data model
type Component struct {
	ID     int `json:"id"`
	Name   string `json:"name"`
	Status int `json:"status"`
}

// LoadCurrentIncident - Returns current incident
func (comp *Component) LoadCurrentIncident(cfg *CachetMonitor) (*Incident, error) {
	resp, body, err := cfg.API.NewRequest("GET", "/incidents?component_id="+strconv.Itoa(comp.ID), []byte(""))

	if err != nil || resp.StatusCode != 200 {
		return nil, err
	}

	incidentInfoA := []Incident{}

	if e := json.Unmarshal(body.Data, &incidentInfoA); e != nil {
	        fmt.Printf("Error decoding JSON: %v\n", e)
	}

	if len(incidentInfoA) == 0 {
		return nil, err
	}

	return &incidentInfoA[0], err
}