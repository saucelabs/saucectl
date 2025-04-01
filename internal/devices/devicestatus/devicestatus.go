package devicestatus

import (
	"fmt"
	"strings"
)

type Status string

const (
	Unknown     Status = "UNKNOWN"
	Available          = "AVAILABLE"
	InUse              = "IN_USE"
	Cleaning           = "CLEANING"
	Maintenance        = "MAINTENANCE"
	Rebooting          = "REBOOTING"
	Offline            = "OFFLINE"
)

var stringToStatusMap = map[string]Status{
	string(Unknown): Unknown,
	Available:       Available,
	InUse:           InUse,
	Cleaning:        Cleaning,
	Maintenance:     Maintenance,
	Rebooting:       Rebooting,
	Offline:         Offline,
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
