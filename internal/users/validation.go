package users

import (
	"net/url"
	"regexp"
	"strings"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/google/uuid"
)

var usernamePattern = regexp.MustCompile(`^[a-z0-9_.-]{3,30}$`)

func validateVisibility(value string, field string) []response.ErrorDetail {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "" {
		return nil
	}

	switch normalized {
	case VisibilityPublic, VisibilityPrivate, VisibilityFollowers:
		return nil
	default:
		return []response.ErrorDetail{{Field: field, Message: "visibility must be public, private or followers"}}
	}
}

func validateURLString(raw string, field string) []response.ErrorDetail {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return []response.ErrorDetail{{Field: field, Message: "must be a valid http or https URL"}}
	}

	return nil
}

func validateUpdateProfileInput(input UpdateUserProfileInput) error {
	details := make([]response.ErrorDetail, 0)

	if input.FullName != nil {
		fullName := strings.TrimSpace(*input.FullName)
		if len(fullName) < 2 || len(fullName) > 80 {
			details = append(details, response.ErrorDetail{Field: "full_name", Message: "must be between 2 and 80 characters"})
		}
	}

	if input.Bio != nil {
		bio := strings.TrimSpace(*input.Bio)
		if len(bio) > 320 {
			details = append(details, response.ErrorDetail{Field: "bio", Message: "must be up to 320 characters"})
		}
	}

	if input.AvatarURL != nil {
		details = append(details, validateURLString(*input.AvatarURL, "avatar_url")...)
	}

	if input.MultimediaVisibility != nil {
		details = append(details, validateVisibility(*input.MultimediaVisibility, "multimedia_visibility")...)
	}

	if input.BadgesVisibility != nil {
		details = append(details, validateVisibility(*input.BadgesVisibility, "badges_visibility")...)
	}

	if len(details) > 0 {
		return ValidationError{Details: details}
	}

	return nil
}

func validateCreateLocationInput(input CreateUserLocationInput) error {
	details := make([]response.ErrorDetail, 0)

	name := strings.TrimSpace(input.Name)
	if len(name) < 2 || len(name) > 80 {
		details = append(details, response.ErrorDetail{Field: "name", Message: "must be between 2 and 80 characters"})
	}

	if input.Latitude < -90 || input.Latitude > 90 {
		details = append(details, response.ErrorDetail{Field: "latitude", Message: "must be between -90 and 90"})
	}

	if input.Longitude < -180 || input.Longitude > 180 {
		details = append(details, response.ErrorDetail{Field: "longitude", Message: "must be between -180 and 180"})
	}

	if input.CountryCode != nil {
		country := strings.TrimSpace(*input.CountryCode)
		if country != "" && len(country) != 2 {
			details = append(details, response.ErrorDetail{Field: "country_code", Message: "must use ISO-3166 alpha-2"})
		}
	}

	if len(details) > 0 {
		return ValidationError{Details: details}
	}

	return nil
}

func validateCreateMediaInput(input CreateUserMediaInput) error {
	details := make([]response.ErrorDetail, 0)

	switch strings.ToLower(strings.TrimSpace(input.MediaType)) {
	case MediaTypeImage, MediaTypeVideo:
	default:
		details = append(details, response.ErrorDetail{Field: "media_type", Message: "must be image or video"})
	}

	details = append(details, validateURLString(input.URL, "url")...)

	if input.ThumbnailURL != nil {
		details = append(details, validateURLString(*input.ThumbnailURL, "thumbnail_url")...)
	}

	if input.Caption != nil && len(strings.TrimSpace(*input.Caption)) > 180 {
		details = append(details, response.ErrorDetail{Field: "caption", Message: "must be up to 180 characters"})
	}

	if len(details) > 0 {
		return ValidationError{Details: details}
	}

	return nil
}

func validateVerifyAchievementInput(input VerifyAchievementInput) error {
	details := make([]response.ErrorDetail, 0)

	if input.LocationID == uuid.Nil {
		details = append(details, response.ErrorDetail{Field: "location_id", Message: "is required"})
	}
	if input.BadgeID == uuid.Nil {
		details = append(details, response.ErrorDetail{Field: "badge_id", Message: "is required"})
	}

	if len(details) > 0 {
		return ValidationError{Details: details}
	}

	return nil
}

func isValidUsername(username string) bool {
	return usernamePattern.MatchString(strings.ToLower(strings.TrimSpace(username)))
}
