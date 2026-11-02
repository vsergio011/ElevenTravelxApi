package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "golang.org/x/image/webp"

	"github.com/eleventravel/eleventravel-api/internal/http/response"
)

type Settings struct {
	PublicBucket  string
	PrivateBucket string
	PublicDomain  string
}

type Service struct {
	repository Repository
	store      ObjectStore
	settings   Settings
	now        func() time.Time
}

func NewService(repository Repository, store ObjectStore, settings Settings) *Service {
	return &Service{repository: repository, store: store, settings: settings, now: time.Now}
}

func (s *Service) bucketFor(visibility string) string {
	if visibility == VisibilityPublic {
		return s.settings.PublicBucket
	}
	return s.settings.PrivateBucket
}

func (s *Service) publicURL(key string) string {
	return s.settings.PublicDomain + "/" + key
}

func (s *Service) CreateUpload(ctx context.Context, userID uuid.UUID, input CreateUploadInput) (UploadTicket, error) {
	mime := strings.ToLower(strings.TrimSpace(input.MimeType))
	purpose := strings.TrimSpace(input.Purpose)

	details := make([]response.ErrorDetail, 0)
	rule, purposeOK := purposeRules[purpose]
	if !purposeOK {
		details = append(details, response.ErrorDetail{Field: "purpose", Message: "must be avatar or profile_media"})
	}
	extension, mimeOK := allowedMimeTypes[mime]
	if !mimeOK {
		details = append(details, response.ErrorDetail{Field: "mime_type", Message: "must be image/jpeg, image/png or image/webp"})
	}
	if input.SizeBytes <= 0 || (purposeOK && input.SizeBytes > rule.maxBytes) {
		details = append(details, response.ErrorDetail{Field: "size_bytes", Message: "must be greater than zero and within the allowed limit"})
	}
	if len(details) > 0 {
		return UploadTicket{}, ValidationError{Details: details}
	}

	visibility := VisibilityPublic
	if purpose == PurposeProfileMedia {
		visibility = VisibilityPrivate
	}

	pending, err := s.repository.CountPending(ctx, userID)
	if err != nil {
		return UploadTicket{}, err
	}
	if pending >= maxPendingPerUser {
		return UploadTicket{}, ErrTooManyPending
	}

	id := uuid.New()
	image := Image{
		ID:                id,
		OwnerUserID:       userID,
		Purpose:           purpose,
		Visibility:        visibility,
		Bucket:            s.bucketFor(visibility),
		ObjectKey:         fmt.Sprintf("%s/%s/%s.%s", purpose, userID, id, extension),
		MimeType:          mime,
		DeclaredSizeBytes: input.SizeBytes,
		ExpiresAt:         s.now().Add(UploadURLTTL),
	}

	uploadURL, err := s.store.PresignPut(ctx, image.Bucket, image.ObjectKey, mime, UploadURLTTL)
	if err != nil {
		return UploadTicket{}, err
	}

	stored, err := s.repository.CreatePending(ctx, image)
	if err != nil {
		return UploadTicket{}, err
	}

	return UploadTicket{Image: stored, UploadURL: uploadURL, Headers: map[string]string{"Content-Type": mime}}, nil
}

func (s *Service) Complete(ctx context.Context, userID uuid.UUID, imageID uuid.UUID) (Image, string, error) {
	img, err := s.repository.Get(ctx, imageID)
	if err != nil {
		return Image{}, "", err
	}
	if img.OwnerUserID != userID {
		return Image{}, "", ErrForbidden
	}

	if img.Status != StatusCompleted {
		img, err = s.verifyAndComplete(ctx, img)
		if err != nil {
			return Image{}, "", err
		}
	}

	if img.Purpose == PurposeAvatar {
		avatarURL := s.publicURL(img.ObjectKey)
		if err := s.repository.SetAvatarURL(ctx, userID, &avatarURL); err != nil {
			return Image{}, "", err
		}
		s.removeOtherAvatars(ctx, userID, img.ID)
	}

	return img, s.urlFor(ctx, img), nil
}

// verifyAndComplete checks the real object (size, format, dimensions) and never trusts the client-declared MIME type.
func (s *Service) verifyAndComplete(ctx context.Context, img Image) (Image, error) {
	rule := purposeRules[img.Purpose]

	info, err := s.store.Head(ctx, img.Bucket, img.ObjectKey)
	if errors.Is(err, ErrObjectNotFound) {
		return Image{}, ErrUploadMissing
	}
	if err != nil {
		return Image{}, err
	}
	if info.Size <= 0 || info.Size > rule.maxBytes {
		return Image{}, s.reject(ctx, img, "size_bytes", "uploaded file exceeds the allowed size")
	}

	data, err := s.store.Get(ctx, img.Bucket, img.ObjectKey, rule.maxBytes)
	if err != nil {
		return Image{}, err
	}

	config, format, decodeErr := image.DecodeConfig(bytes.NewReader(data))
	if decodeErr != nil {
		return Image{}, s.reject(ctx, img, "file", "uploaded file is not a valid image")
	}
	if "image/"+format != img.MimeType {
		return Image{}, s.reject(ctx, img, "mime_type", "uploaded content does not match the declared type")
	}
	if config.Width < 1 || config.Height < 1 || config.Width > maxImageSide || config.Height > maxImageSide {
		return Image{}, s.reject(ctx, img, "file", "image dimensions are not allowed")
	}

	return s.repository.MarkCompleted(ctx, img.ID, int64(len(data)), config.Width, config.Height)
}

func (s *Service) reject(ctx context.Context, img Image, field string, message string) error {
	s.removeImage(ctx, img)
	return ValidationError{Details: []response.ErrorDetail{{Field: field, Message: message}}}
}

func (s *Service) removeImage(ctx context.Context, img Image) {
	if err := s.store.Delete(ctx, img.Bucket, img.ObjectKey); err != nil {
		log.Printf("storage: delete object %s failed: %v", img.ObjectKey, err)
		return
	}
	if err := s.repository.Delete(ctx, img.ID); err != nil {
		log.Printf("storage: delete image %s failed: %v", img.ID, err)
	}
}

func (s *Service) removeOtherAvatars(ctx context.Context, userID uuid.UUID, keepID uuid.UUID) {
	others, err := s.repository.ListOtherAvatars(ctx, userID, keepID)
	if err != nil {
		log.Printf("storage: list old avatars failed: %v", err)
		return
	}
	for _, other := range others {
		s.removeImage(ctx, other)
	}
}

func (s *Service) urlFor(ctx context.Context, img Image) string {
	if img.Visibility == VisibilityPublic {
		return s.publicURL(img.ObjectKey)
	}
	return ""
}

func (s *Service) GetURL(ctx context.Context, actorID uuid.UUID, imageID uuid.UUID) (ImageURL, error) {
	img, err := s.repository.Get(ctx, imageID)
	if err != nil {
		return ImageURL{}, err
	}
	if img.Status != StatusCompleted {
		return ImageURL{}, ErrNotFound
	}

	if img.OwnerUserID != actorID && img.Purpose != PurposeAvatar {
		visibility, err := s.repository.MultimediaVisibility(ctx, img.OwnerUserID)
		if err != nil {
			return ImageURL{}, err
		}
		allowed := visibility == VisibilityPublic
		if visibility == "followers" {
			allowed, err = s.repository.IsFollowing(ctx, actorID, img.OwnerUserID)
			if err != nil {
				return ImageURL{}, err
			}
		}
		if !allowed {
			return ImageURL{}, ErrForbidden
		}
	}

	if img.Visibility == VisibilityPublic {
		return ImageURL{URL: s.publicURL(img.ObjectKey)}, nil
	}

	signed, err := s.store.PresignGet(ctx, img.Bucket, img.ObjectKey, DownloadURLTTL)
	if err != nil {
		return ImageURL{}, err
	}
	expiresAt := s.now().Add(DownloadURLTTL)
	return ImageURL{URL: signed, ExpiresAt: &expiresAt}, nil
}

func (s *Service) Delete(ctx context.Context, actorID uuid.UUID, imageID uuid.UUID) error {
	img, err := s.repository.Get(ctx, imageID)
	if err != nil {
		return err
	}
	if img.OwnerUserID != actorID {
		return ErrForbidden
	}

	if err := s.store.Delete(ctx, img.Bucket, img.ObjectKey); err != nil {
		return err
	}
	if err := s.repository.DeleteMediaForImage(ctx, img.ID); err != nil {
		return err
	}
	if err := s.repository.Delete(ctx, img.ID); err != nil {
		return err
	}
	if img.Purpose == PurposeAvatar {
		if err := s.repository.SetAvatarURL(ctx, actorID, nil); err != nil && !errors.Is(err, ErrProfileMissing) {
			return err
		}
	}

	return nil
}

// CleanupExpired removes pending uploads that were never completed.
func (s *Service) CleanupExpired(ctx context.Context) {
	expired, err := s.repository.ListExpiredPending(ctx, s.now(), 100)
	if err != nil {
		log.Printf("storage: list expired uploads failed: %v", err)
		return
	}
	for _, img := range expired {
		s.removeImage(ctx, img)
	}
}

func (s *Service) RunCleanup(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.CleanupExpired(ctx)
		}
	}
}

// AttachMediaImage validates that a completed profile_media image belongs to the user and returns its stored URL.
func (s *Service) AttachMediaImage(ctx context.Context, userID uuid.UUID, imageID uuid.UUID) (string, error) {
	img, err := s.repository.Get(ctx, imageID)
	if err != nil {
		return "", err
	}
	if img.OwnerUserID != userID {
		return "", ErrForbidden
	}
	if img.Purpose != PurposeProfileMedia || img.Status != StatusCompleted {
		return "", ValidationError{Details: []response.ErrorDetail{{Field: "image_id", Message: "must reference a completed profile_media image"}}}
	}

	return s.urlFor(ctx, img), nil
}

// ResolveURLs returns a displayable URL (public or short-lived signed) for each completed image.
func (s *Service) ResolveURLs(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID]string, error) {
	urls := make(map[uuid.UUID]string, len(ids))
	if len(ids) == 0 {
		return urls, nil
	}

	images, err := s.repository.ListByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	for _, img := range images {
		if img.Visibility == VisibilityPublic {
			urls[img.ID] = s.publicURL(img.ObjectKey)
			continue
		}
		signed, signErr := s.store.PresignGet(ctx, img.Bucket, img.ObjectKey, DownloadURLTTL)
		if signErr != nil {
			return nil, signErr
		}
		urls[img.ID] = signed
	}

	return urls, nil
}

// MigratePublicProfileMedia moves profile media stored in the public bucket to the private bucket.
// Profile media is always private: access is granted per request through signed URLs.
func (s *Service) MigratePublicProfileMedia(ctx context.Context) (int, error) {
	images, err := s.repository.ListPublicProfileMedia(ctx)
	if err != nil {
		return 0, err
	}
	moved := 0
	for _, img := range images {
		destination := s.settings.PrivateBucket
		if err := s.store.Copy(ctx, img.Bucket, destination, img.ObjectKey); err != nil {
			return moved, fmt.Errorf("copy %s: %w", img.ObjectKey, err)
		}
		if err := s.repository.UpdateBucket(ctx, img.ID, destination, VisibilityPrivate); err != nil {
			return moved, fmt.Errorf("update image %s: %w", img.ID, err)
		}
		if err := s.store.Delete(ctx, img.Bucket, img.ObjectKey); err != nil {
			log.Printf("storage: delete migrated object %s failed: %v", img.ObjectKey, err)
		}
		moved++
	}

	return moved, nil
}
