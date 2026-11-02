package users

import (
	"context"
	"errors"
	"log"
	"strings"

	"github.com/google/uuid"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
	"github.com/eleventravel/eleventravel-api/internal/storage"
)

type Service struct {
	repository Repository
	images     ImageStore
}

// ImageStore is the storage capability users needs to link uploaded images to profile media.
type ImageStore interface {
	AttachMediaImage(ctx context.Context, userID uuid.UUID, imageID uuid.UUID) (string, error)
	ResolveURLs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error)
	Delete(ctx context.Context, actorID uuid.UUID, imageID uuid.UUID) error
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) WithImageStore(images ImageStore) *Service {
	s.images = images
	return s
}

func translateStorageError(err error) error {
	var validationErr storage.ValidationError
	switch {
	case errors.As(err, &validationErr):
		return ValidationError{Details: validationErr.Details}
	case errors.Is(err, storage.ErrNotFound):
		return ValidationError{Details: []response.ErrorDetail{{Field: "image_id", Message: "image not found"}}}
	case errors.Is(err, storage.ErrForbidden):
		return ErrForbidden
	default:
		return err
	}
}

// IsUsernameAvailable reports whether no profile currently uses the username (case-insensitive).
func (s *Service) IsUsernameAvailable(ctx context.Context, username string) (bool, error) {
	_, err := s.repository.GetProfileByUsername(ctx, username)
	if errors.Is(err, ErrProfileNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	return false, nil
}

// CreateRegisteredProfile creates the profile for a newly registered account. It returns ErrConflict if the username is taken.
func (s *Service) CreateRegisteredProfile(ctx context.Context, userID uuid.UUID, username string) error {
	fullName := "Traveler " + strings.ToUpper(userID.String()[:6])
	_, err := s.repository.CreateProfile(ctx, userID, username, fullName)
	return err
}

func (s *Service) GetMyProfile(ctx context.Context, actorUserID uuid.UUID) (UserProfileView, error) {
	profile, err := s.repository.EnsureProfile(ctx, actorUserID, "")
	if err != nil {
		return UserProfileView{}, err
	}

	return s.buildProfileView(ctx, actorUserID, profile)
}

func (s *Service) GetProfileByUsername(ctx context.Context, actorUserID uuid.UUID, username string) (UserProfileView, error) {
	profile, err := s.repository.GetProfileByUsername(ctx, username)
	if err != nil {
		return UserProfileView{}, err
	}

	return s.buildProfileView(ctx, actorUserID, profile)
}

func (s *Service) UpdateMyProfile(ctx context.Context, actorUserID uuid.UUID, input UpdateUserProfileInput) (UserProfileView, error) {
	if err := validateUpdateProfileInput(input); err != nil {
		return UserProfileView{}, err
	}

	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return UserProfileView{}, err
	}

	profile, err := s.repository.UpdateProfile(ctx, actorUserID, input)
	if err != nil {
		return UserProfileView{}, err
	}

	return s.buildProfileView(ctx, actorUserID, profile)
}

func (s *Service) FollowUser(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error {
	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return err
	}
	if _, err := s.repository.EnsureProfile(ctx, targetUserID, ""); err != nil {
		return err
	}
	return s.repository.Follow(ctx, actorUserID, targetUserID)
}

func (s *Service) UnfollowUser(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID) error {
	return s.repository.Unfollow(ctx, actorUserID, targetUserID)
}

func (s *Service) ListFollowers(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, limit int) ([]UserSummary, error) {
	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return nil, err
	}
	if _, err := s.repository.EnsureProfile(ctx, targetUserID, ""); err != nil {
		return nil, err
	}
	return s.repository.ListFollowers(ctx, targetUserID, clampLimit(limit, 100))
}

func (s *Service) ListFollowing(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, limit int) ([]UserSummary, error) {
	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return nil, err
	}
	if _, err := s.repository.EnsureProfile(ctx, targetUserID, ""); err != nil {
		return nil, err
	}
	return s.repository.ListFollowing(ctx, targetUserID, clampLimit(limit, 100))
}

func (s *Service) SearchUsers(ctx context.Context, actorUserID uuid.UUID, query string, limit int) ([]UserSearchResult, error) {
	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return nil, err
	}

	return s.repository.SearchUsers(ctx, actorUserID, strings.TrimSpace(query), clampLimit(limit, 50))
}

func (s *Service) AddLocation(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, input CreateUserLocationInput) (UserLocationPin, error) {
	if actorUserID != targetUserID {
		return UserLocationPin{}, ErrForbidden
	}

	if err := validateCreateLocationInput(input); err != nil {
		return UserLocationPin{}, err
	}

	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return UserLocationPin{}, err
	}

	return s.repository.CreateLocation(ctx, actorUserID, input)
}

func (s *Service) UpdateLocation(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, locationID uuid.UUID, input UpdateUserLocationInput) (UserLocationPin, error) {
	if actorUserID != targetUserID {
		return UserLocationPin{}, ErrForbidden
	}

	if err := validateCreateLocationInput(CreateUserLocationInput{Name: input.Name, Latitude: input.Latitude, Longitude: input.Longitude, CountryCode: input.CountryCode, VisitedAt: input.VisitedAt, VisitedUntil: input.VisitedUntil}); err != nil {
		return UserLocationPin{}, err
	}

	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return UserLocationPin{}, err
	}

	return s.repository.UpdateLocation(ctx, actorUserID, locationID, input)
}

func (s *Service) DeleteLocation(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, locationID uuid.UUID) error {
	if actorUserID != targetUserID {
		return ErrForbidden
	}

	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return err
	}

	return s.repository.DeleteLocation(ctx, actorUserID, locationID)
}

func (s *Service) AddMedia(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, input CreateUserMediaInput) (UserMediaAsset, error) {
	if actorUserID != targetUserID {
		return UserMediaAsset{}, ErrForbidden
	}

	if err := validateCreateMediaInput(input); err != nil {
		return UserMediaAsset{}, err
	}

	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return UserMediaAsset{}, err
	}

	if input.ImageID != nil {
		if s.images == nil {
			return UserMediaAsset{}, ValidationError{Details: []response.ErrorDetail{{Field: "image_id", Message: "image storage is not available"}}}
		}
		imageURL, err := s.images.AttachMediaImage(ctx, actorUserID, *input.ImageID)
		if err != nil {
			return UserMediaAsset{}, translateStorageError(err)
		}
		input.URL = imageURL
	} else if strings.TrimSpace(input.URL) == "" {
		return UserMediaAsset{}, ValidationError{Details: []response.ErrorDetail{{Field: "url", Message: "url or image_id is required"}}}
	}

	item, err := s.repository.CreateMedia(ctx, actorUserID, input)
	if err != nil {
		return UserMediaAsset{}, err
	}

	return s.resolveMediaURLs(ctx, []UserMediaAsset{item})[0], nil
}

func (s *Service) UpdateMedia(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, mediaID uuid.UUID, input UpdateUserMediaInput) (UserMediaAsset, error) {
	if actorUserID != targetUserID {
		return UserMediaAsset{}, ErrForbidden
	}

	if input.Caption != nil && len(strings.TrimSpace(*input.Caption)) > 180 {
		return UserMediaAsset{}, ValidationError{Details: []response.ErrorDetail{{Field: "caption", Message: "must be up to 180 characters"}}}
	}

	item, err := s.repository.UpdateMedia(ctx, actorUserID, mediaID, input)
	if err != nil {
		return UserMediaAsset{}, err
	}

	return s.resolveMediaURLs(ctx, []UserMediaAsset{item})[0], nil
}

func (s *Service) DeleteMedia(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, mediaID uuid.UUID) error {
	if actorUserID != targetUserID {
		return ErrForbidden
	}

	imageID, err := s.repository.DeleteMedia(ctx, actorUserID, mediaID)
	if err != nil {
		return err
	}

	if imageID != nil && s.images != nil {
		if err := s.images.Delete(ctx, actorUserID, *imageID); err != nil && !errors.Is(err, storage.ErrNotFound) {
			log.Printf("users: delete media image failed: %v", err)
		}
	}

	return nil
}

// resolveMediaURLs replaces stored URLs with public or short-lived signed URLs for storage-backed media.
func (s *Service) resolveMediaURLs(ctx context.Context, items []UserMediaAsset) []UserMediaAsset {
	if s.images == nil {
		return items
	}

	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		if item.ImageID != nil {
			ids = append(ids, *item.ImageID)
		}
	}
	if len(ids) == 0 {
		return items
	}

	urls, err := s.images.ResolveURLs(ctx, ids)
	if err != nil {
		log.Printf("users: resolve media urls failed: %v", err)
		urls = map[uuid.UUID]string{}
	}
	for index := range items {
		if items[index].ImageID == nil {
			continue
		}
		items[index].URL = urls[*items[index].ImageID]
	}

	return items
}

func (s *Service) VerifyAchievement(ctx context.Context, actorUserID uuid.UUID, targetUserID uuid.UUID, input VerifyAchievementInput) (UserBadge, error) {
	if actorUserID != targetUserID {
		return UserBadge{}, ErrForbidden
	}

	if err := validateVerifyAchievementInput(input); err != nil {
		return UserBadge{}, err
	}

	if _, err := s.repository.EnsureProfile(ctx, actorUserID, ""); err != nil {
		return UserBadge{}, err
	}

	return s.repository.UpsertVerifiedAchievement(ctx, actorUserID, input)
}

func (s *Service) buildProfileView(ctx context.Context, actorUserID uuid.UUID, profile UserProfile) (UserProfileView, error) {
	stats, err := s.repository.GetFollowStats(ctx, actorUserID, profile.UserID)
	if err != nil {
		return UserProfileView{}, err
	}

	pins, err := s.repository.ListLocationPins(ctx, profile.UserID)
	if err != nil {
		return UserProfileView{}, err
	}

	mediaCanView := canViewSection(profile.MultimediaVisibility, stats.IsMe, stats.IsFollowing)
	badgesCanView := canViewSection(profile.BadgesVisibility, stats.IsMe, stats.IsFollowing)

	mediaItems := make([]UserMediaAsset, 0)
	if mediaCanView {
		mediaItems, err = s.repository.ListMedia(ctx, profile.UserID, 60)
		if err != nil {
			return UserProfileView{}, err
		}
		mediaItems = s.resolveMediaURLs(ctx, mediaItems)
	}

	badgeItems := make([]UserBadge, 0)
	if badgesCanView {
		badgeItems, err = s.repository.ListBadges(ctx, profile.UserID, 60)
		if err != nil {
			return UserProfileView{}, err
		}
	}

	return UserProfileView{
		Profile: profile,
		Stats:   stats,
		MapPins: pins,
		Media: UserSection[UserMediaAsset]{
			Visibility: profile.MultimediaVisibility,
			CanView:    mediaCanView,
			Items:      mediaItems,
		},
		Badges: UserSection[UserBadge]{
			Visibility: profile.BadgesVisibility,
			CanView:    badgesCanView,
			Items:      badgeItems,
		},
	}, nil
}

func canViewSection(visibility string, isMe bool, isFollowing bool) bool {
	if isMe {
		return true
	}

	switch strings.ToLower(strings.TrimSpace(visibility)) {
	case VisibilityPublic:
		return true
	case VisibilityFollowers:
		return isFollowing
	case VisibilityPrivate:
		return false
	default:
		return false
	}
}

func clampLimit(value int, max int) int {
	if value <= 0 {
		return 20
	}
	if value > max {
		return max
	}
	return value
}
