package users

import (
	"time"

	"github.com/google/uuid"
)

type UpdateUserProfileRequest struct {
	FullName             *string `json:"full_name"`
	Bio                  *string `json:"bio"`
	AvatarURL            *string `json:"avatar_url"`
	MultimediaVisibility *string `json:"multimedia_visibility"`
	BadgesVisibility     *string `json:"badges_visibility"`
}

type CreateUserLocationRequest struct {
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	CountryCode *string `json:"country_code"`
	VisitedAt   *string `json:"visited_at"`
}

type UpdateUserLocationRequest struct {
	Name        string  `json:"name"`
	Latitude    float64 `json:"latitude"`
	Longitude   float64 `json:"longitude"`
	CountryCode *string `json:"country_code"`
}

type CreateUserMediaRequest struct {
	LocationID   string  `json:"location_id"`
	MediaType    string  `json:"media_type"`
	URL          string  `json:"url"`
	ThumbnailURL *string `json:"thumbnail_url"`
	Caption      *string `json:"caption"`
}

type VerifyAchievementRequest struct {
	LocationID string `json:"location_id"`
	BadgeID    string `json:"badge_id"`
}

type UserProfileResponse struct {
	UserID         uuid.UUID                                   `json:"user_id"`
	Username       string                                      `json:"username"`
	FullName       string                                      `json:"full_name"`
	Bio            *string                                     `json:"bio"`
	AvatarURL      *string                                     `json:"avatar_url"`
	FollowersCount int                                         `json:"followers_count"`
	FollowingCount int                                         `json:"following_count"`
	IsFollowing    bool                                        `json:"is_following"`
	IsMe           bool                                        `json:"is_me"`
	MapPins        []UserLocationPinResponse                   `json:"map_pins"`
	Multimedia     UserSectionResponse[UserMediaAssetResponse] `json:"multimedia"`
	Badges         UserSectionResponse[UserBadgeResponse]      `json:"badges"`
}

type UserSectionResponse[T any] struct {
	Visibility string `json:"visibility"`
	CanView    bool   `json:"can_view"`
	Items      []T    `json:"items"`
}

type UserLocationPinResponse struct {
	LocationID               uuid.UUID `json:"location_id"`
	Name                     string    `json:"name"`
	Latitude                 float64   `json:"latitude"`
	Longitude                float64   `json:"longitude"`
	CountryCode              *string   `json:"country_code"`
	ThumbnailURL             *string   `json:"thumbnail_url"`
	MediaCount               int       `json:"media_count"`
	HasVerifiedAchievement   bool      `json:"has_verified_achievement"`
	VerifiedAchievementCount int       `json:"verified_achievement_count"`
}

type UserMediaAssetResponse struct {
	ID           uuid.UUID `json:"id"`
	LocationID   uuid.UUID `json:"location_id"`
	MediaType    string    `json:"media_type"`
	URL          string    `json:"url"`
	ThumbnailURL *string   `json:"thumbnail_url"`
	Caption      *string   `json:"caption"`
	CreatedAt    time.Time `json:"created_at"`
}

type UserBadgeResponse struct {
	ID           uuid.UUID `json:"id"`
	BadgeID      uuid.UUID `json:"badge_id"`
	Code         string    `json:"code"`
	Title        string    `json:"title"`
	IconURL      *string   `json:"icon_url"`
	LocationID   uuid.UUID `json:"location_id"`
	LocationName string    `json:"location_name"`
	UnlockedAt   time.Time `json:"unlocked_at"`
}

type UserSummaryResponse struct {
	UserID     uuid.UUID `json:"user_id"`
	Username   string    `json:"username"`
	FullName   string    `json:"full_name"`
	AvatarURL  *string   `json:"avatar_url"`
	FollowedAt time.Time `json:"followed_at"`
}

type UserSearchResultResponse struct {
	UserID      uuid.UUID `json:"user_id"`
	Username    string    `json:"username"`
	FullName    string    `json:"full_name"`
	AvatarURL   *string   `json:"avatar_url"`
	IsFollowing bool      `json:"is_following"`
	IsMe        bool      `json:"is_me"`
}

func toProfileResponse(view UserProfileView) UserProfileResponse {
	pins := make([]UserLocationPinResponse, 0, len(view.MapPins))
	for _, pin := range view.MapPins {
		pins = append(pins, UserLocationPinResponse{
			LocationID:               pin.LocationID,
			Name:                     pin.Name,
			Latitude:                 pin.Latitude,
			Longitude:                pin.Longitude,
			CountryCode:              pin.CountryCode,
			ThumbnailURL:             pin.ThumbnailURL,
			MediaCount:               pin.MediaCount,
			HasVerifiedAchievement:   pin.HasVerifiedAchievement,
			VerifiedAchievementCount: pin.VerifiedAchievementCount,
		})
	}

	mediaItems := make([]UserMediaAssetResponse, 0, len(view.Media.Items))
	for _, item := range view.Media.Items {
		mediaItems = append(mediaItems, UserMediaAssetResponse{
			ID:           item.ID,
			LocationID:   item.LocationID,
			MediaType:    item.MediaType,
			URL:          item.URL,
			ThumbnailURL: item.ThumbnailURL,
			Caption:      item.Caption,
			CreatedAt:    item.CreatedAt,
		})
	}

	badgeItems := make([]UserBadgeResponse, 0, len(view.Badges.Items))
	for _, item := range view.Badges.Items {
		badgeItems = append(badgeItems, UserBadgeResponse{
			ID:           item.ID,
			BadgeID:      item.BadgeID,
			Code:         item.Code,
			Title:        item.Title,
			IconURL:      item.IconURL,
			LocationID:   item.LocationID,
			LocationName: item.LocationName,
			UnlockedAt:   item.UnlockedAt,
		})
	}

	return UserProfileResponse{
		UserID:         view.Profile.UserID,
		Username:       view.Profile.Username,
		FullName:       view.Profile.FullName,
		Bio:            view.Profile.Bio,
		AvatarURL:      view.Profile.AvatarURL,
		FollowersCount: view.Stats.FollowersCount,
		FollowingCount: view.Stats.FollowingCount,
		IsFollowing:    view.Stats.IsFollowing,
		IsMe:           view.Stats.IsMe,
		MapPins:        pins,
		Multimedia: UserSectionResponse[UserMediaAssetResponse]{
			Visibility: view.Media.Visibility,
			CanView:    view.Media.CanView,
			Items:      mediaItems,
		},
		Badges: UserSectionResponse[UserBadgeResponse]{
			Visibility: view.Badges.Visibility,
			CanView:    view.Badges.CanView,
			Items:      badgeItems,
		},
	}
}

func toUserSummaryResponse(items []UserSummary) []UserSummaryResponse {
	result := make([]UserSummaryResponse, 0, len(items))
	for _, item := range items {
		result = append(result, UserSummaryResponse{
			UserID:     item.UserID,
			Username:   item.Username,
			FullName:   item.FullName,
			AvatarURL:  item.AvatarURL,
			FollowedAt: item.FollowedAt,
		})
	}

	return result
}

func toUserSearchResultResponse(items []UserSearchResult) []UserSearchResultResponse {
	result := make([]UserSearchResultResponse, 0, len(items))
	for _, item := range items {
		result = append(result, UserSearchResultResponse{
			UserID: item.UserID, Username: item.Username, FullName: item.FullName, AvatarURL: item.AvatarURL,
			IsFollowing: item.IsFollowing, IsMe: item.IsMe,
		})
	}

	return result
}
