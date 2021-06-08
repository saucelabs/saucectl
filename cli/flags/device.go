package flags

import (
	"encoding/csv"
	"fmt"
	"github.com/saucelabs/saucectl/internal/config"
	"strconv"
	"strings"
)

// Device represents the RDC device configuration.
type Device struct {
	config.Device
	Changed bool
}

// String returns a string represenation of the device.
func (d Device) String() string {
	return fmt.Sprintf("%v", d.Device)
}

// Set sets the device to the values present in s.
// The input has to be a comma separated string in the format of "key=value,key2=value2".
// This method is called by cobra when CLI flags are parsed.
func (d *Device) Set(s string) error {
	d.Changed = true

	rec, err := csv.NewReader(strings.NewReader(s)).Read()
	if err != nil {
		return err
	}

	for _, v := range rec {
		vs := strings.Split(v, "=")
		val := vs[1]
		switch vs[0] {
		case "id":
			d.ID = val
		case "name":
			d.Name = val
		case "platformName":
			d.PlatformName = val
		case "platformVersion":
			d.PlatformVersion = val
		case "carrierConnectivity":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			d.Options.CarrierConnectivity = b
		case "deviceType":
			d.Options.DeviceType = val
		case "private":
			b, err := strconv.ParseBool(val)
			if err != nil {
				return err
			}
			d.Options.Private = b
		}
	}

	return nil
}

// Type returns the value type.
func (d Device) Type() string {
	return "device"
}
