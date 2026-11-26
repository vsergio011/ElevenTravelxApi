package notifications

import "time"

type NotificationActorResponse struct {
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	FullName  string  `json:"full_name"`
	AvatarURL *string `json:"avatar_url,omitempty"`
}

type NotificationResponse struct {
	ID        string                     `json:"id"`
	Type      string                     `json:"type"`
	Title     string                     `json:"title"`
	Body      string                     `json:"body"`
	Data      map[string]any             `json:"data"`
	ReadAt    *string                    `json:"read_at,omitempty"`
	CreatedAt string                     `json:"created_at"`
	Actor     *NotificationActorResponse `json:"actor,omitempty"`
}

type NotificationListResponse struct {
	Items       []NotificationResponse `json:"items"`
	UnreadCount int                    `json:"unread_count"`
}

type UnreadCountResponse struct {
	UnreadCount int `json:"unread_count"`
}

type PushSubscriptionKeysRequest struct {
	P256DH string `json:"p256dh"`
	Auth   string `json:"auth"`
}

type UpsertPushSubscriptionRequest struct {
	Endpoint string                      `json:"endpoint"`
	Keys     PushSubscriptionKeysRequest `json:"keys"`
}

type DeletePushSubscriptionRequest struct {
	Endpoint string `json:"endpoint"`
}

type PushConfigResponse struct {
	Enabled        bool   `json:"enabled"`
	VAPIDPublicKey string `json:"vapid_public_key,omitempty"`
}

func toNotificationResponse(item Notification) NotificationResponse {
	data := item.Data
	if data == nil {
		data = map[string]any{}
	}

	resp := NotificationResponse{
		ID:        item.ID.String(),
		Type:      item.Type,
		Title:     item.Title,
		Body:      item.Body,
		Data:      data,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
	}

	if item.ReadAt != nil {
		formatted := item.ReadAt.UTC().Format(time.RFC3339)
		resp.ReadAt = &formatted
	}

	if item.Actor != nil {
		resp.Actor = &NotificationActorResponse{
			UserID:    item.Actor.UserID.String(),
			Username:  item.Actor.Username,
			FullName:  item.Actor.FullName,
			AvatarURL: item.Actor.AvatarURL,
		}
	}

	return resp
}

func toNotificationListResponse(result ListResult) NotificationListResponse {
	items := make([]NotificationResponse, 0, len(result.Items))
	for _, item := range result.Items {
		items = append(items, toNotificationResponse(item))
	}

	return NotificationListResponse{
		Items:       items,
		UnreadCount: result.UnreadCount,
	}
}
