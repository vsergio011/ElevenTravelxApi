package storage

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"
	"time"

	"github.com/google/uuid"
)

type fakeStore struct {
	objects map[string][]byte
	deleted []string
}

func (f *fakeStore) PresignPut(_ context.Context, bucket string, key string, _ string, _ time.Duration) (string, error) {
	return "https://put/" + bucket + "/" + key, nil
}
func (f *fakeStore) PresignGet(_ context.Context, bucket string, key string, _ time.Duration) (string, error) {
	return "https://get/" + bucket + "/" + key, nil
}
func (f *fakeStore) Head(_ context.Context, bucket string, key string) (ObjectInfo, error) {
	data, ok := f.objects[bucket+"/"+key]
	if !ok {
		return ObjectInfo{}, ErrObjectNotFound
	}
	return ObjectInfo{Size: int64(len(data))}, nil
}
func (f *fakeStore) Get(_ context.Context, bucket string, key string, _ int64) ([]byte, error) {
	return f.objects[bucket+"/"+key], nil
}
func (f *fakeStore) Copy(_ context.Context, srcBucket string, dstBucket string, key string) error {
	data, ok := f.objects[srcBucket+"/"+key]
	if !ok {
		return ErrObjectNotFound
	}
	f.objects[dstBucket+"/"+key] = data
	return nil
}
func (f *fakeStore) Delete(_ context.Context, bucket string, key string) error {
	f.deleted = append(f.deleted, bucket+"/"+key)
	delete(f.objects, bucket+"/"+key)
	return nil
}

type fakeRepo struct {
	Repository
	images map[uuid.UUID]Image
	avatar *string
}

func (r *fakeRepo) MultimediaVisibility(context.Context, uuid.UUID) (string, error) {
	return VisibilityPrivate, nil
}
func (r *fakeRepo) CountPending(context.Context, uuid.UUID) (int, error) { return 0, nil }
func (r *fakeRepo) CreatePending(_ context.Context, img Image) (Image, error) {
	r.images[img.ID] = img
	return img, nil
}
func (r *fakeRepo) Get(_ context.Context, id uuid.UUID) (Image, error) {
	img, ok := r.images[id]
	if !ok {
		return Image{}, ErrNotFound
	}
	return img, nil
}
func (r *fakeRepo) MarkCompleted(_ context.Context, id uuid.UUID, size int64, w int, h int) (Image, error) {
	img := r.images[id]
	img.Status, img.SizeBytes, img.Width, img.Height = StatusCompleted, &size, &w, &h
	r.images[id] = img
	return img, nil
}
func (r *fakeRepo) Delete(_ context.Context, id uuid.UUID) error { delete(r.images, id); return nil }
func (r *fakeRepo) SetAvatarURL(_ context.Context, _ uuid.UUID, url *string) error {
	r.avatar = url
	return nil
}
func (r *fakeRepo) ListOtherAvatars(context.Context, uuid.UUID, uuid.UUID) ([]Image, error) {
	return nil, nil
}
func (r *fakeRepo) ListPublicProfileMedia(context.Context) ([]Image, error) {
	var out []Image
	for _, img := range r.images {
		if img.Purpose == PurposeProfileMedia && img.Visibility == VisibilityPublic && img.Status == StatusCompleted {
			out = append(out, img)
		}
	}
	return out, nil
}
func (r *fakeRepo) UpdateBucket(_ context.Context, id uuid.UUID, bucket string, visibility string) error {
	img := r.images[id]
	img.Bucket, img.Visibility = bucket, visibility
	r.images[id] = img
	return nil
}

func newTestService() (*Service, *fakeStore, *fakeRepo) {
	store := &fakeStore{objects: map[string][]byte{}}
	repo := &fakeRepo{images: map[uuid.UUID]Image{}}
	service := NewService(repo, store, Settings{PublicBucket: "pub", PrivateBucket: "priv", PublicDomain: "https://media.test"})
	return service, store, repo
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 4, 3))
	img.Set(0, 0, color.White)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestCreateUploadRejectsInvalidInput(t *testing.T) {
	service, _, _ := newTestService()
	_, err := service.CreateUpload(context.Background(), uuid.New(), CreateUploadInput{Purpose: "x", MimeType: "image/gif", SizeBytes: 0})
	var validationErr ValidationError
	if !errors.As(err, &validationErr) || len(validationErr.Details) != 3 {
		t.Fatalf("expected 3 validation details, got %v", err)
	}
}

func TestCompleteAvatarUsesPublicBucketAndSetsAvatar(t *testing.T) {
	service, store, repo := newTestService()
	userID := uuid.New()
	ticket, err := service.CreateUpload(context.Background(), userID, CreateUploadInput{Purpose: PurposeAvatar, MimeType: "image/png", SizeBytes: 100})
	if err != nil || ticket.Image.Bucket != "pub" {
		t.Fatalf("unexpected ticket: %v %v", ticket.Image.Bucket, err)
	}

	store.objects["pub/"+ticket.Image.ObjectKey] = pngBytes(t)
	img, url, err := service.Complete(context.Background(), userID, ticket.Image.ID)
	if err != nil || img.Status != StatusCompleted || *img.Width != 4 || *img.Height != 3 {
		t.Fatalf("complete failed: %v %+v", err, img)
	}
	if url != "https://media.test/"+img.ObjectKey || repo.avatar == nil || *repo.avatar != url {
		t.Fatalf("avatar url not set: %q", url)
	}
}

func TestCompleteRejectsContentThatDoesNotMatchDeclaredMime(t *testing.T) {
	service, store, repo := newTestService()
	userID := uuid.New()
	ticket, _ := service.CreateUpload(context.Background(), userID, CreateUploadInput{Purpose: PurposeProfileMedia, MimeType: "image/jpeg", SizeBytes: 100})
	if ticket.Image.Bucket != "priv" {
		t.Fatalf("private visibility must use private bucket, got %s", ticket.Image.Bucket)
	}

	store.objects["priv/"+ticket.Image.ObjectKey] = pngBytes(t)
	_, _, err := service.Complete(context.Background(), userID, ticket.Image.ID)
	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected validation error, got %v", err)
	}
	if len(store.deleted) != 1 || len(repo.images) != 0 {
		t.Fatal("rejected upload must be removed")
	}
}

func TestMigratePublicProfileMediaMovesObjectsToPrivateBucket(t *testing.T) {
	service, store, repo := newTestService()
	id := uuid.New()
	repo.images[id] = Image{ID: id, Purpose: PurposeProfileMedia, Status: StatusCompleted, Visibility: VisibilityPublic, Bucket: "pub", ObjectKey: "media/a.png"}
	store.objects["pub/media/a.png"] = []byte("data")

	moved, err := service.MigratePublicProfileMedia(context.Background())
	if err != nil || moved != 1 {
		t.Fatalf("migration failed: moved=%d err=%v", moved, err)
	}
	if string(store.objects["priv/media/a.png"]) != "data" || repo.images[id].Bucket != "priv" || repo.images[id].Visibility != VisibilityPrivate {
		t.Fatalf("object not moved: %+v", repo.images[id])
	}
	if _, ok := store.objects["pub/media/a.png"]; ok {
		t.Fatal("public original must be deleted")
	}
}

func TestCompleteRequiresUploadAndOwner(t *testing.T) {
	service, _, _ := newTestService()
	userID := uuid.New()
	ticket, _ := service.CreateUpload(context.Background(), userID, CreateUploadInput{Purpose: PurposeAvatar, MimeType: "image/png", SizeBytes: 100})

	if _, _, err := service.Complete(context.Background(), uuid.New(), ticket.Image.ID); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if _, _, err := service.Complete(context.Background(), userID, ticket.Image.ID); !errors.Is(err, ErrUploadMissing) {
		t.Fatalf("expected upload missing, got %v", err)
	}
}
