package devicestatus

import (
	"fmt"
	"strings"
)

type Status string

const (
	Unknown     Status = "UNKNOWN"
	Available   Status = "AVAILABLE"
	InUse       Status = "IN_USE"
	Cleaning    Status = "CLEANING"
	Maintenance Status = "MAINTENANCE"
	Rebooting   Status = "REBOOTING"
	Offline     Status = "OFFLINE"
)

var stringToStatusMap = map[string]Status{
	string(Unknown):     Unknown,
	string(Available):   Available,
	string(InUse):       InUse,
	string(Cleaning):    Cleaning,
	string(Maintenance): Maintenance,
	string(Rebooting):   Rebooting,
	string(Offline):     Offline,
}

func Make(str string) (Status, error) {
	c, ok := stringToStatusMap[strings.ToUpper(str)]
	if !ok {
		return c, fmt.Errorf("unknown status %s", str)
	}

	return c, nil
}

func (ds Status) String() string {
	return string(ds)
}
