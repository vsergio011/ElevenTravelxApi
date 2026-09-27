package flights

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type FlightLookup struct {
	Airline      string          `json:"airline"`
	FlightNumber string          `json:"flight_number"`
	Status       string          `json:"status"`
	Aircraft     string          `json:"aircraft,omitempty"`
	Segments     []LookupSegment `json:"segments"`
}

type LookupSegment struct {
	OriginCode        string `json:"origin_code"`
	OriginName        string `json:"origin_name"`
	OriginCity        string `json:"origin_city"`
	DestinationCode   string `json:"destination_code"`
	DestinationName   string `json:"destination_name"`
	DestinationCity   string `json:"destination_city"`
	DepartureAt       string `json:"departure_at"`
	ArrivalAt         string `json:"arrival_at"`
	DepartureTerminal string `json:"departure_terminal,omitempty"`
	ArrivalTerminal   string `json:"arrival_terminal,omitempty"`
	DepartureGate     string `json:"departure_gate,omitempty"`
	ArrivalGate       string `json:"arrival_gate,omitempty"`
}

type FlightLookupProvider interface {
	Lookup(context.Context, string, time.Time) (FlightLookup, error)
}

type AeroDataBoxClient struct {
	baseURL    string
	apiKey     string
	host       string
	httpClient *http.Client
}

func NewAeroDataBoxClient(apiKey, host string) *AeroDataBoxClient {
	return newAeroDataBoxClient("https://"+host, apiKey, host, &http.Client{Timeout: 8 * time.Second})
}

func newAeroDataBoxClient(baseURL, apiKey, host string, httpClient *http.Client) *AeroDataBoxClient {
	return &AeroDataBoxClient{baseURL: strings.TrimSuffix(baseURL, "/"), apiKey: apiKey, host: host, httpClient: httpClient}
}

func (c *AeroDataBoxClient) Lookup(ctx context.Context, flightNumber string, flightDate time.Time) (FlightLookup, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return FlightLookup{}, ErrFlightLookupUnavailable
	}

	endpoint := fmt.Sprintf(
		"%s/flights/number/%s/%s",
		c.baseURL,
		url.PathEscape(strings.ToUpper(strings.TrimSpace(flightNumber))),
		flightDate.Format("2006-01-02"),
	)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return FlightLookup{}, err
	}
	request.Header.Set("X-RapidAPI-Key", c.apiKey)
	request.Header.Set("X-RapidAPI-Host", c.host)

	response, err := c.httpClient.Do(request)
	if err != nil {
		return FlightLookup{}, err
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return FlightLookup{}, ErrFlightLookupNotFound
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return FlightLookup{}, ErrFlightLookupUnavailable
	}

	var flights []aeroDataBoxFlight
	if err := json.NewDecoder(response.Body).Decode(&flights); err != nil {
		return FlightLookup{}, err
	}
	if len(flights) == 0 {
		return FlightLookup{}, ErrFlightLookupNotFound
	}
	return toFlightLookup(flights[0]), nil
}

type aeroDataBoxFlight struct {
	Number    string                  `json:"number"`
	Status    string                  `json:"status"`
	Airline   aeroDataBoxAirline      `json:"airline"`
	Aircraft  aeroDataBoxAircraft     `json:"aircraft"`
	Departure aeroDataBoxAirportEvent `json:"departure"`
	Arrival   aeroDataBoxAirportEvent `json:"arrival"`
}

type aeroDataBoxAirline struct {
	Name string `json:"name"`
}

type aeroDataBoxAircraft struct {
	Model string `json:"model"`
}

type aeroDataBoxAirportEvent struct {
	Airport       aeroDataBoxAirport `json:"airport"`
	ScheduledTime aeroDataBoxTime    `json:"scheduledTime"`
	RevisedTime   aeroDataBoxTime    `json:"revisedTime"`
	Terminal      string             `json:"terminal"`
	Gate          string             `json:"gate"`
}

type aeroDataBoxAirport struct {
	IATA             string `json:"iata"`
	ICAO             string `json:"icao"`
	Name             string `json:"name"`
	MunicipalityName string `json:"municipalityName"`
}

type aeroDataBoxTime struct {
	Local string `json:"local"`
	UTC   string `json:"utc"`
}

func toFlightLookup(source aeroDataBoxFlight) FlightLookup {
	return FlightLookup{
		Airline:      source.Airline.Name,
		FlightNumber: source.Number,
		Status:       source.Status,
		Aircraft:     source.Aircraft.Model,
		Segments: []LookupSegment{{
			OriginCode:        airportCode(source.Departure.Airport),
			OriginName:        source.Departure.Airport.Name,
			OriginCity:        source.Departure.Airport.MunicipalityName,
			DestinationCode:   airportCode(source.Arrival.Airport),
			DestinationName:   source.Arrival.Airport.Name,
			DestinationCity:   source.Arrival.Airport.MunicipalityName,
			DepartureAt:       eventTime(source.Departure),
			ArrivalAt:         eventTime(source.Arrival),
			DepartureTerminal: source.Departure.Terminal,
			ArrivalTerminal:   source.Arrival.Terminal,
			DepartureGate:     source.Departure.Gate,
			ArrivalGate:       source.Arrival.Gate,
		}},
	}
}

func airportCode(airport aeroDataBoxAirport) string {
	if airport.IATA != "" {
		return airport.IATA
	}
	return airport.ICAO
}

func eventTime(event aeroDataBoxAirportEvent) string {
	if event.RevisedTime.Local != "" {
		return normalizeAeroDataBoxTime(event.RevisedTime.Local)
	}
	if event.ScheduledTime.Local != "" {
		return normalizeAeroDataBoxTime(event.ScheduledTime.Local)
	}
	if event.RevisedTime.UTC != "" {
		return normalizeAeroDataBoxTime(event.RevisedTime.UTC)
	}
	return normalizeAeroDataBoxTime(event.ScheduledTime.UTC)
}

func normalizeAeroDataBoxTime(value string) string {
	for _, layout := range []string{time.RFC3339, "2006-01-02 15:04Z07:00"} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.Format(time.RFC3339)
		}
	}
	return value
}
