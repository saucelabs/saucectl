package devicestatus

import "fmt"

type Status int

const (
	Available Status = iota
	InUse
	Cleaning
	Maintenance
	Rebooting
	Offline
)

var statusToStringMap = map[Status]string{
	Available:   "AVAILABLE",
	InUse:       "IN_USE",
	Cleaning:    "CLEANING",
	Maintenance: "MAINTENANCE",
	Rebooting:   "REBOOTING",
	Offline:     "OFFLINE",
}

var stringToStatusMap = map[string]Status{
	"AVAILABLE":   Available,
	"IN_USE":      InUse,
	"CLEANING":    Cleaning,
	"MAINTENANCE": Maintenance,
	"REBOOTING":   Rebooting,
	"OFFLINE":     Offline,
}

func StrToStatus(status string) (Status, error) {
	c, ok := stringToStatusMap[status]
	if !ok {
		return c, fmt.Errorf("unknown status %s", status)
	}

	return c, nil
}

func StatusToStr(ds Status) string {
	return statusToStringMap[ds]
}

func (ds Status) String() string {
	return StatusToStr(ds)
}
