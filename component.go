package cachet

// Component Cachet data model
type Component struct {
	ID     int `json:"id"`
	Name   string `json:"name"`
	Status int `json:"status"`
}
