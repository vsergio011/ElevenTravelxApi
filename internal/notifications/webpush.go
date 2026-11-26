package notifications

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	webpush "github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
)

type PushSubscriptionStore interface {
	ListPushSubscriptionsForUser(ctx context.Context, userID uuid.UUID) ([]PushSubscription, error)
	DeletePushSubscriptionByEndpoint(ctx context.Context, endpoint string) error
}

type WebPushSettings struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	Subscriber      string
}

type WebPushDispatcher struct {
	store    PushSubscriptionStore
	settings WebPushSettings
}

func NewWebPushDispatcher(store PushSubscriptionStore, settings WebPushSettings) *WebPushDispatcher {
	subscriber := strings.TrimSpace(settings.Subscriber)
	subscriber = strings.TrimPrefix(subscriber, "mailto:")
	if !strings.HasPrefix(subscriber, "https://") && !strings.Contains(subscriber, "@") {
		subscriber = "notifications@eleventravel.app"
	}

	return &WebPushDispatcher{
		store: store,
		settings: WebPushSettings{
			VAPIDPublicKey:  strings.TrimSpace(settings.VAPIDPublicKey),
			VAPIDPrivateKey: strings.TrimSpace(settings.VAPIDPrivateKey),
			Subscriber:      subscriber,
		},
	}
}

func (d *WebPushDispatcher) PublicKey() string {
	return d.settings.VAPIDPublicKey
}

type webPushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Route string `json:"route"`
	Tag   string `json:"tag"`
}

func (d *WebPushDispatcher) Dispatch(ctx context.Context, notification Notification) error {
	subscriptions, err := d.store.ListPushSubscriptionsForUser(ctx, notification.RecipientUserID)
	if err != nil {
		return err
	}
	if len(subscriptions) == 0 {
		log.Printf("notifications: skip webpush for user %s (no active push subscriptions)", notification.RecipientUserID)
		return nil
	}

	route := "/notificaciones"
	if notification.Data != nil {
		if rawRoute, ok := notification.Data["route"].(string); ok && strings.TrimSpace(rawRoute) != "" {
			route = strings.TrimSpace(rawRoute)
		}
	}

	rawPayload, err := json.Marshal(webPushPayload{
		Title: notification.Title,
		Body:  notification.Body,
		Route: route,
		Tag:   "eleventravel-" + notification.ID.String(),
	})
	if err != nil {
		return err
	}

	options := &webpush.Options{
		Subscriber:      d.settings.Subscriber,
		VAPIDPublicKey:  d.settings.VAPIDPublicKey,
		VAPIDPrivateKey: d.settings.VAPIDPrivateKey,
		TTL:             3600,
	}

	for _, sub := range subscriptions {
		resp, sendErr := webpush.SendNotificationWithContext(ctx, rawPayload, &webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				P256dh: sub.P256DH,
				Auth:   sub.Auth,
			},
		}, options)
		if sendErr != nil {
			log.Printf("notifications: webpush send error for user %s: %v", notification.RecipientUserID, sendErr)
			continue
		}

		statusCode := resp.StatusCode
		_ = resp.Body.Close()

		if statusCode == http.StatusNotFound || statusCode == http.StatusGone {
			if delErr := d.store.DeletePushSubscriptionByEndpoint(ctx, sub.Endpoint); delErr != nil {
				log.Printf("notifications: delete stale push subscription failed: %v", delErr)
			}
			continue
		}

		if statusCode >= 400 {
			log.Printf("notifications: webpush rejected with status %d for user %s", statusCode, notification.RecipientUserID)
			continue
		}

		log.Printf("notifications: webpush delivered (status %d) for user %s", statusCode, notification.RecipientUserID)
	}

	return nil
}
