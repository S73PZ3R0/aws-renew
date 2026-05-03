package network

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const ipAPIURL = "https://api.ipify.org?format=json"

func FetchPublicIP() (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(ipAPIURL)
	if err != nil {
		return "", fmt.Errorf("network error fetching public IP: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("network error reading IP response: %w", err)
	}
	var data struct{ IP string `json:"ip"` }
	if err := json.Unmarshal(body, &data); err != nil {
		return "", fmt.Errorf("invalid IP response: %w", err)
	}
	return data.IP, nil
}

func NormalizeIP(value string) (string, error) {
	if strings.Contains(value, "/") {
		return value, nil
	}
	ip := net.ParseIP(value)
	if ip == nil {
		return "", fmt.Errorf("invalid IP address: %s", value)
	}
	if ip.To4() != nil {
		return value + "/32", nil
	}
	return value + "/128", nil
}
