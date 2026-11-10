package main

import (
	"fmt"
	"strings"
)

// robotSerial returns the serial number of the gateway's nth simulated
// robot. It is derived from the gatewayId, so a restarted simulator scans
// the same robots and serial numbers stay unique across gateways.
func robotSerial(gatewayID string, n int) string {
	return fmt.Sprintf("SIM-%s-R%03d", strings.ToUpper(gatewayID[:min(8, len(gatewayID))]), n)
}
