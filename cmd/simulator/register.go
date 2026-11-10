package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

// register registers a new gateway through the REST API, as an operator
// would before installing it, and returns its gatewayId.
func register(ctx context.Context, baseURL, location string) (string, error) {
	body, err := json.Marshal(map[string]string{
		"serialNumber": "SIM-GW-" + strings.ToUpper(uuid.NewString()[:8]),
		"location":     location,
		"name":         "Simulated gateway",
	})
	if err != nil {
		return "", fmt.Errorf("register gateway: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimSuffix(baseURL, "/")+"/v1/gateways", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("register gateway: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("register gateway: %w", err)
	}
	defer func() { _ = res.Body.Close() }() // nothing to do if closing a read body fails
	if res.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("register gateway: unexpected status %s", res.Status)
	}
	var created struct {
		GatewayID string `json:"gatewayId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		return "", fmt.Errorf("register gateway: decode response: %w", err)
	}
	return created.GatewayID, nil
}

// robotSerial returns the serial number of the gateway's nth simulated
// robot. It is derived from the gatewayId, so a restarted simulator scans
// the same robots and serial numbers stay unique across gateways.
func robotSerial(gatewayID string, n int) string {
	return fmt.Sprintf("SIM-%s-R%03d", strings.ToUpper(gatewayID[:min(8, len(gatewayID))]), n)
}
