package users

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

type Service struct {
	repository Repository
}

func NewService(repository Repository) *Service {
	return &Service{repository: repository}
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

	if err := validateCreateLocationInput(CreateUserLocationInput{Name: input.Name, Latitude: input.Latitude, Longitude: input.Longitude, CountryCode: input.CountryCode}); err != nil {
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

	return s.repository.CreateMedia(ctx, actorUserID, input)
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
