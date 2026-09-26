package flights

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestAeroDataBoxClientLookupMapsScheduledFlight(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		require.Equal(t, "/flights/number/IB3166/2026-10-20", request.URL.Path)
		require.Equal(t, "test-key", request.Header.Get("X-RapidAPI-Key"))
		require.Equal(t, "aerodatabox.p.rapidapi.com", request.Header.Get("X-RapidAPI-Host"))
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`[
			{
				"number":"IB3166",
				"status":"Scheduled",
				"airline":{"name":"Iberia"},
				"aircraft":{"model":"Airbus A320"},
				"departure":{"airport":{"iata":"MAD","name":"Adolfo Suarez Madrid-Barajas","municipalityName":"Madrid"},"scheduledTime":{"local":"2026-10-20 08:15+02:00"},"terminal":"4","gate":"J42"},
				"arrival":{"airport":{"iata":"LPA","name":"Gran Canaria","municipalityName":"Las Palmas"},"scheduledTime":{"local":"2026-10-20 10:10+01:00"},"terminal":"1","gate":"C16"}
			}
		]`))
	}))
	defer server.Close()

	client := newAeroDataBoxClient(server.URL, "test-key", "aerodatabox.p.rapidapi.com", server.Client())
	flight, err := client.Lookup(context.Background(), "ib3166", time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC))

	require.NoError(t, err)
	require.Equal(t, "Iberia", flight.Airline)
	require.Equal(t, "IB3166", flight.FlightNumber)
	require.Equal(t, "Scheduled", flight.Status)
	require.Equal(t, "Airbus A320", flight.Aircraft)
	require.Len(t, flight.Segments, 1)
	require.Equal(t, "MAD", flight.Segments[0].OriginCode)
	require.Equal(t, "LPA", flight.Segments[0].DestinationCode)
	require.Equal(t, "2026-10-20T08:15:00+02:00", flight.Segments[0].DepartureAt)
	require.Equal(t, "4", flight.Segments[0].DepartureTerminal)
	require.Equal(t, "C16", flight.Segments[0].ArrivalGate)
}

func TestAeroDataBoxClientLookupRequiresPrivateKey(t *testing.T) {
	client := newAeroDataBoxClient("https://aerodatabox.p.rapidapi.com", "", "aerodatabox.p.rapidapi.com", http.DefaultClient)

	_, err := client.Lookup(context.Background(), "IB3166", time.Now())

	require.ErrorIs(t, err, ErrFlightLookupUnavailable)
}
