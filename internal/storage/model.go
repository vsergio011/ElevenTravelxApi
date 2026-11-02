package storage

import (
	"errors"
	"time"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

const (
	PurposeAvatar       = "avatar"
	PurposeProfileMedia = "profile_media"

	VisibilityPublic  = "public"
	VisibilityPrivate = "private"

	StatusPending   = "pending"
	StatusCompleted = "completed"

	UploadURLTTL      = 5 * time.Minute
	DownloadURLTTL    = 15 * time.Minute
	maxPendingPerUser = 20
	maxImageSide      = 4096
)

var (
	ErrNotFound       = errors.New("image not found")
	ErrForbidden      = errors.New("forbidden")
	ErrUploadMissing  = errors.New("uploaded object not found")
	ErrTooManyPending = errors.New("too many pending uploads")
	ErrUnavailable    = errors.New("storage is not configured")
	ErrProfileMissing = errors.New("profile not found")
	ErrObjectNotFound = errors.New("object not found")
)

type ValidationError struct {
	Details []response.ErrorDetail
}

func (e ValidationError) Error() string { return "validation error" }

type Image struct {
	ID                uuid.UUID
	OwnerUserID       uuid.UUID
	Purpose           string
	Visibility        string
	Bucket            string
	ObjectKey         string
	MimeType          string
	DeclaredSizeBytes int64
	SizeBytes         *int64
	Width             *int
	Height            *int
	Status            string
	ExpiresAt         time.Time
	CreatedAt         time.Time
	CompletedAt       *time.Time
}

type CreateUploadInput struct {
	Purpose   string
	MimeType  string
	SizeBytes int64
}

type UploadTicket struct {
	Image     Image
	UploadURL string
	Headers   map[string]string
}

type ImageURL struct {
	URL       string
	ExpiresAt *time.Time
}

type purposeRule struct {
	maxBytes int64
}

var purposeRules = map[string]purposeRule{
	PurposeAvatar:       {maxBytes: 5 << 20},
	PurposeProfileMedia: {maxBytes: 10 << 20},
}

var allowedMimeTypes = map[string]string{
	"image/jpeg": "jpg",
	"image/png":  "png",
	"image/webp": "webp",
}
