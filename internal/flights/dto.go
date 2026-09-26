package flights

import (
	"time"

	"github.com/google/uuid"
)

type SegmentRequest struct {
	Direction              string    `json:"direction"`
	Position               int       `json:"position"`
	OriginLabel            string    `json:"origin_label"`
	OriginCity             *string   `json:"origin_city"`
	OriginAirportName      *string   `json:"origin_airport_name"`
	OriginAirportCode      *string   `json:"origin_airport_code"`
	OriginMapboxID         *string   `json:"origin_mapbox_id"`
	DestinationLabel       string    `json:"destination_label"`
	DestinationCity        *string   `json:"destination_city"`
	DestinationAirportName *string   `json:"destination_airport_name"`
	DestinationAirportCode *string   `json:"destination_airport_code"`
	DestinationMapboxID    *string   `json:"destination_mapbox_id"`
	DepartureAt            time.Time `json:"departure_at"`
	ArrivalAt              time.Time `json:"arrival_at"`
	DepartureTerminal      *string   `json:"departure_terminal"`
	ArrivalTerminal        *string   `json:"arrival_terminal"`
	Airline                *string   `json:"airline"`
	FlightNumber           *string   `json:"flight_number"`
	OriginTimezone         *string   `json:"origin_timezone"`
	DestinationTimezone    *string   `json:"destination_timezone"`
}

type CreateRequest struct {
	Status             *string          `json:"status"`
	Airline            *string          `json:"airline"`
	FlightNumber       *string          `json:"flight_number"`
	ReservationCode    *string          `json:"reservation_code"`
	Notes              *string          `json:"notes"`
	OfferURL           *string          `json:"offer_url"`
	CabinClass         *string          `json:"cabin_class"`
	Baggage            *string          `json:"baggage"`
	CostCents          *int64           `json:"cost_cents"`
	Currency           *string          `json:"currency"`
	CostDistribution   *string          `json:"cost_distribution"`
	ParticipantUserIDs []uuid.UUID      `json:"participant_user_ids"`
	Segments           []SegmentRequest `json:"segments"`
}
type UpdateRequest struct {
	Status             *string           `json:"status"`
	Airline            *string           `json:"airline"`
	FlightNumber       *string           `json:"flight_number"`
	ReservationCode    *string           `json:"reservation_code"`
	Notes              *string           `json:"notes"`
	OfferURL           *string           `json:"offer_url"`
	CabinClass         *string           `json:"cabin_class"`
	Baggage            *string           `json:"baggage"`
	CostCents          *int64            `json:"cost_cents"`
	Currency           *string           `json:"currency"`
	CostDistribution   *string           `json:"cost_distribution"`
	ParticipantUserIDs *[]uuid.UUID      `json:"participant_user_ids"`
	Segments           *[]SegmentRequest `json:"segments"`
}

type SegmentResponse SegmentRequest
type FlightResponse struct {
	ID                 uuid.UUID         `json:"id"`
	PlanningID         uuid.UUID         `json:"planning_id"`
	CreatedByUserID    uuid.UUID         `json:"created_by_user_id"`
	Status             string            `json:"status"`
	Airline            *string           `json:"airline"`
	FlightNumber       *string           `json:"flight_number"`
	ReservationCode    *string           `json:"reservation_code"`
	Notes              *string           `json:"notes"`
	OfferURL           *string           `json:"offer_url"`
	CabinClass         *string           `json:"cabin_class"`
	Baggage            *string           `json:"baggage"`
	CostCents          int64             `json:"cost_cents"`
	Currency           string            `json:"currency"`
	CostDistribution   string            `json:"cost_distribution"`
	ParticipantUserIDs []uuid.UUID       `json:"participant_user_ids"`
	Segments           []SegmentResponse `json:"segments"`
	CreatedAt          time.Time         `json:"created_at"`
	UpdatedAt          time.Time         `json:"updated_at"`
}

func toSegment(input SegmentRequest) Segment {
	return Segment{Direction: input.Direction, Position: input.Position, OriginLabel: input.OriginLabel, OriginCity: input.OriginCity, OriginAirportName: input.OriginAirportName, OriginAirportCode: input.OriginAirportCode, OriginMapboxID: input.OriginMapboxID, DestinationLabel: input.DestinationLabel, DestinationCity: input.DestinationCity, DestinationAirportName: input.DestinationAirportName, DestinationAirportCode: input.DestinationAirportCode, DestinationMapboxID: input.DestinationMapboxID, DepartureAt: input.DepartureAt, ArrivalAt: input.ArrivalAt, DepartureTerminal: input.DepartureTerminal, ArrivalTerminal: input.ArrivalTerminal, Airline: input.Airline, FlightNumber: input.FlightNumber, OriginTimezone: input.OriginTimezone, DestinationTimezone: input.DestinationTimezone}
}
func toSegments(input []SegmentRequest) []Segment {
	result := make([]Segment, len(input))
	for i, item := range input {
		result[i] = toSegment(item)
	}
	return result
}
func toResponse(flight Flight) FlightResponse {
	segments := make([]SegmentResponse, len(flight.Segments))
	for i, item := range flight.Segments {
		segments[i] = SegmentResponse{Direction: item.Direction, Position: item.Position, OriginLabel: item.OriginLabel, OriginCity: item.OriginCity, OriginAirportName: item.OriginAirportName, OriginAirportCode: item.OriginAirportCode, OriginMapboxID: item.OriginMapboxID, DestinationLabel: item.DestinationLabel, DestinationCity: item.DestinationCity, DestinationAirportName: item.DestinationAirportName, DestinationAirportCode: item.DestinationAirportCode, DestinationMapboxID: item.DestinationMapboxID, DepartureAt: item.DepartureAt, ArrivalAt: item.ArrivalAt, DepartureTerminal: item.DepartureTerminal, ArrivalTerminal: item.ArrivalTerminal, Airline: item.Airline, FlightNumber: item.FlightNumber, OriginTimezone: item.OriginTimezone, DestinationTimezone: item.DestinationTimezone}
	}
	return FlightResponse{ID: flight.ID, PlanningID: flight.PlanningID, CreatedByUserID: flight.CreatedByUserID, Status: flight.Status, Airline: flight.Airline, FlightNumber: flight.FlightNumber, ReservationCode: flight.ReservationCode, Notes: flight.Notes, OfferURL: flight.OfferURL, CabinClass: flight.CabinClass, Baggage: flight.Baggage, CostCents: flight.CostCents, Currency: flight.Currency, CostDistribution: flight.CostDistribution, ParticipantUserIDs: flight.ParticipantUserIDs, Segments: segments, CreatedAt: flight.CreatedAt, UpdatedAt: flight.UpdatedAt}
}
