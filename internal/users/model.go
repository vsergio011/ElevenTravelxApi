package users

import (
	"time"

	"github.com/google/uuid"
)

const (
	VisibilityPublic    = "public"
	VisibilityPrivate   = "private"
	VisibilityFollowers = "followers"
)

const (
	MediaTypeImage = "image"
	MediaTypeVideo = "video"
)

type UserProfile struct {
	UserID               uuid.UUID
	Username             string
	FullName             string
	Bio                  *string
	AvatarURL            *string
	MultimediaVisibility string
	BadgesVisibility     string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type UserFollowStats struct {
	FollowersCount int
	FollowingCount int
	IsFollowing    bool
	IsMe           bool
}

type UserSummary struct {
	UserID     uuid.UUID
	Username   string
	FullName   string
	AvatarURL  *string
	FollowedAt time.Time
}

type UserLocationPin struct {
	LocationID               uuid.UUID
	Name                     string
	Latitude                 float64
	Longitude                float64
	CountryCode              *string
	ThumbnailURL             *string
	MediaCount               int
	HasVerifiedAchievement   bool
	VerifiedAchievementCount int
}

type UserMediaAsset struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	LocationID   uuid.UUID
	MediaType    string
	URL          string
	ThumbnailURL *string
	Caption      *string
	CreatedAt    time.Time
}

type UserBadge struct {
	ID           uuid.UUID
	BadgeID      uuid.UUID
	Code         string
	Title        string
	IconURL      *string
	UnlockedAt   time.Time
	LocationID   uuid.UUID
	LocationName string
}

type UserAchievement struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	BadgeID    uuid.UUID
	LocationID uuid.UUID
	ObtainedAt time.Time
	CreatedAt  time.Time
}

type UserProfileView struct {
	Profile UserProfile
	Stats   UserFollowStats
	MapPins []UserLocationPin
	Media   UserSection[UserMediaAsset]
	Badges  UserSection[UserBadge]
}

type UserSection[T any] struct {
	Visibility string
	CanView    bool
	Items      []T
}

type UpdateUserProfileInput struct {
	FullName             *string
	Bio                  *string
	AvatarURL            *string
	MultimediaVisibility *string
	BadgesVisibility     *string
}

type CreateUserLocationInput struct {
	Name        string
	Latitude    float64
	Longitude   float64
	CountryCode *string
	VisitedAt   *time.Time
}

type UpdateUserLocationInput struct {
	Name        string
	Latitude    float64
	Longitude   float64
	CountryCode *string
}

type CreateUserMediaInput struct {
	LocationID   uuid.UUID
	MediaType    string
	URL          string
	ThumbnailURL *string
	Caption      *string
}

type VerifyAchievementInput struct {
	LocationID uuid.UUID
	BadgeID    uuid.UUID
}
