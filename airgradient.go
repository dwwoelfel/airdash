package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

type FlexibleFloat64 float64

func (ff *FlexibleFloat64) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	switch val := v.(type) {
	case float64:
		*ff = FlexibleFloat64(val)
	case string:
		if val == "" {
			*ff = 0
			return nil
		}
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			return err
		}
		*ff = FlexibleFloat64(f)
	case nil:
		*ff = 0
	default:
		return fmt.Errorf("invalid type for float64: %T", v)
	}
	return nil
}

type AirGradientMeasures struct {
	LocationID         int             `json:"locationId"`
	LocationName       string          `json:"locationName"`
	Pm01               FlexibleFloat64 `json:"pm01"`
	Pm02               FlexibleFloat64 `json:"pm02"`
	Pm10               FlexibleFloat64 `json:"pm10"`
	Pm003Count         FlexibleFloat64 `json:"pm003Count"`
	Atmp               FlexibleFloat64 `json:"atmp"`
	Rhum               FlexibleFloat64 `json:"rhum"`
	Rco2               FlexibleFloat64 `json:"rco2"`
	Tvoc               FlexibleFloat64 `json:"tvoc"`
	Wifi               FlexibleFloat64 `json:"wifi"`
	Timestamp          time.Time       `json:"timestamp"`
	LedMode            string          `json:"ledMode"`
	LedCo2Threshold1   FlexibleFloat64 `json:"ledCo2Threshold1"`
	LedCo2Threshold2   FlexibleFloat64 `json:"ledCo2Threshold2"`
	LedCo2ThresholdEnd FlexibleFloat64 `json:"ledCo2ThresholdEnd"`
	Serialno           string          `json:"serialno"`
	FirmwareVersion    string          `json:"firmwareVersion"`
	TvocIndex          FlexibleFloat64 `json:"tvocIndex"`
	NoxIndex           FlexibleFloat64 `json:"noxIndex"`
}

const airGradientAPIBaseURL = "https://api.airgradient.com/public/api/v1"

var (
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	ErrBadPayload = errors.New("error unmarshalling JSON")
)

// getAirGradientAPIURL returns the AirGradient API URL.
func getAirGradientAPIURL(locationID int) string {
	if locationID != 0 {
		return fmt.Sprintf("%s/locations/%d/measures/current", airGradientAPIBaseURL, locationID)
	}
	return fmt.Sprintf("%s/locations/measures/current", airGradientAPIBaseURL)
}

// convertTemperature converts the temperature from Celsius to Fahrenheit if the
// temperature unit is set to Fahrenheit.
// By default the temperature unit is Celsius.
func convertTemperature(temperature float64, tempUnit string) float64 {
	if tempUnit == "F" {
		return (temperature * 9 / 5) + 32
	}
	return temperature
}

// fetchMeasures fetches the measures from the AirGradient API.
func fetchMeasures(locationID int, token string) ([]byte, error) {
	apiURL := getAirGradientAPIURL(locationID)
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		logger.Error("Creating HTTP request", "error", err)
		return nil, err
	}

	q := req.URL.Query()
	q.Add("token", token)
	req.URL.RawQuery = q.Encode()

	resp, err := httpClient.Do(req)
	if err != nil {
		logger.Error("Sending HTTP request", "error", err)
		return nil, err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error("Closing response body", "error", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		logger.Error("HTTP request failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("HTTP %d from API", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Reading HTTP request", "error", err)
		return nil, err
	}

	return body, nil
}

func getAirGradientMeasures(locationID int, token string) (AirGradientMeasures, error) {
	var measures AirGradientMeasures
	payload, err := fetchMeasures(locationID, token)
	if err != nil {
		return measures, err
	}

	// Try to unmarshal as a single object first
	if err := json.Unmarshal(payload, &measures); err == nil {
		return measures, nil
	}

	// If that failed, try as an array
	var arrayMeasures []AirGradientMeasures
	if err := json.Unmarshal(payload, &arrayMeasures); err == nil && len(arrayMeasures) > 0 {
		return arrayMeasures[0], nil
	}

	return measures, ErrBadPayload
}
