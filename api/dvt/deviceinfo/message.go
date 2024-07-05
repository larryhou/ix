package deviceinfo

import "time"

type Process struct {
	BundleIdentifier string    `json:"bundleIdentifier"`
	IsApplication    bool      `json:"isApplication"`
	Name             string    `json:"name"`
	Pid              int       `json:"pid"`
	RealAppName      string    `json:"realAppName"`
	StartDate        time.Time `json:"startDate"`
}
