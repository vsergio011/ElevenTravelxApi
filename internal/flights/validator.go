package flights

import (
	"fmt"
	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/google/uuid"
	"sort"
	"strings"
)

func validateInput(input CreateInput) error {
	details := []response.ErrorDetail{}
	if input.CostCents < 0 {
		details = append(details, response.ErrorDetail{Field: "cost_cents", Message: "cost_cents cannot be negative"})
	}
	if input.Status != StatusDraft && input.Status != StatusConfirmed {
		details = append(details, response.ErrorDetail{Field: "status", Message: "status must be draft or confirmed"})
	}
	if input.Currency != "EUR" && input.Currency != "USD" {
		details = append(details, response.ErrorDetail{Field: "currency", Message: "currency must be EUR or USD"})
	}
	if !validDistribution(input.CostDistribution) {
		details = append(details, response.ErrorDetail{Field: "cost_distribution", Message: "invalid cost distribution"})
	}
	if input.CostDistribution == DistributionSelectedParticipants && len(input.ParticipantUserIDs) == 0 {
		details = append(details, response.ErrorDetail{Field: "participant_user_ids", Message: "at least one participant is required"})
	}
	if err := validateSegments(input.Segments); err != nil {
		details = append(details, response.ErrorDetail{Field: "segments", Message: err.Error()})
	}
	if err := validateParticipantIDs(input.ParticipantUserIDs); err != nil {
		details = append(details, response.ErrorDetail{Field: "participant_user_ids", Message: err.Error()})
	}
	if len(details) > 0 {
		return newValidationError(details)
	}
	return nil
}

func validateSegments(segments []Segment) error {
	if len(segments) == 0 {
		return fmt.Errorf("at least one segment is required")
	}
	byDirection := map[string][]Segment{}
	for _, segment := range segments {
		if segment.Direction != DirectionOutbound && segment.Direction != DirectionReturn {
			return fmt.Errorf("invalid segment direction")
		}
		if strings.TrimSpace(segment.OriginLabel) == "" || strings.TrimSpace(segment.DestinationLabel) == "" {
			return fmt.Errorf("segment locations are required")
		}
		if segment.ArrivalAt.Before(segment.DepartureAt) || segment.ArrivalAt.Equal(segment.DepartureAt) {
			return fmt.Errorf("arrival must be after departure")
		}
		byDirection[segment.Direction] = append(byDirection[segment.Direction], segment)
	}
	for _, itinerary := range byDirection {
		sort.Slice(itinerary, func(i, j int) bool { return itinerary[i].Position < itinerary[j].Position })
		for position, segment := range itinerary {
			if segment.Position != position {
				return fmt.Errorf("segment positions must be contiguous")
			}
			if position == 0 {
				continue
			}
			previous := itinerary[position-1]
			if segment.DepartureAt.Before(previous.ArrivalAt) {
				return fmt.Errorf("segments must be chronological")
			}
			if !sameAirport(previous.DestinationAirportCode, segment.OriginAirportCode, previous.DestinationLabel, segment.OriginLabel) {
				return fmt.Errorf("segments must connect to the previous destination")
			}
		}
	}
	return nil
}

func sameAirport(previousCode, nextCode *string, previousLabel, nextLabel string) bool {
	if previousCode != nil && nextCode != nil && strings.TrimSpace(*previousCode) != "" && strings.TrimSpace(*nextCode) != "" {
		return strings.EqualFold(strings.TrimSpace(*previousCode), strings.TrimSpace(*nextCode))
	}
	return strings.EqualFold(strings.TrimSpace(previousLabel), strings.TrimSpace(nextLabel))
}

func validDistribution(value string) bool {
	return value == DistributionIndividual || value == DistributionGroup || value == DistributionSelectedParticipants
}

func validateParticipantIDs(ids []uuid.UUID) error {
	for _, id := range ids {
		if id == uuid.Nil {
			return fmt.Errorf("participant ids must be valid UUIDs")
		}
	}
	return nil
}

type responseValidationError struct{ details []response.ErrorDetail }

func (e responseValidationError) Error() string                   { return "invalid flight" }
func (e responseValidationError) Details() []response.ErrorDetail { return e.details }
func newValidationError(details []response.ErrorDetail) responseValidationError {
	return responseValidationError{details: details}
}
