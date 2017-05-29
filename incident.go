package cachet

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// Incident Cachet data model
type Incident struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Message string `json:"message"`
	Status  int    `json:"status"`
	Visible int    `json"visible"`
	Notify  bool   `json:"notify"`

	ComponentID     int `json:"component_id"`
	ComponentStatus int `json:"component_status"`
}

// Send - Create or Update incident
func (incident *Incident) Send(cfg *CachetMonitor) error {
	switch incident.Status {
		case 1, 2, 3:
			// partial outage
			incident.ComponentStatus = 3

			compInfo := cfg.API.GetComponentData(incident.ComponentID)
			if compInfo.Status == 3 {
				// major outage
				incident.ComponentStatus = 4
			}
		case 4:
			// fixed
			incident.ComponentStatus = 1
	}

	requestType := "POST"
	requestURL := "/incidents"
	if incident.ID > 0 {
		requestType = "PUT"
		requestURL += "/" + strconv.Itoa(incident.ID)
	}

	jsonBytes, _ := json.Marshal(incident)

	resp, body, err := cfg.API.NewRequest(requestType, requestURL, jsonBytes)
	if err != nil {
		return err
	}

	var data struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(body.Data, &data); err != nil {
		return fmt.Errorf("Cannot parse incident body: %v, %v", err, string(body.Data))
	}

	incident.ID = data.ID
	if resp.StatusCode != 200 {
		return fmt.Errorf("Could not create/update incident!")
	}

	return nil
}

// SetInvestigating sets status to Investigating
func (incident *Incident) SetInvestigating() {
	incident.Status = 1
}

// SetIdentified sets status to Identified
func (incident *Incident) SetIdentified() {
	incident.Status = 2
}

// SetWatching sets status to Watching
func (incident *Incident) SetWatching() {
	incident.Status = 3
}

// SetFixed sets status to Fixed
func (incident *Incident) SetFixed() {
	incident.Status = 4
}
