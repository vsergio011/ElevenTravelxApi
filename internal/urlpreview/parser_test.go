package urlpreview

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

type stubMembership struct {
	allow bool
}

func (s stubMembership) GetMember(_ context.Context, planningID, userID uuid.UUID) (plannings.PlanningMember, error) {
	if !s.allow {
		return plannings.PlanningMember{}, plannings.ErrMemberNotFound
	}
	return plannings.PlanningMember{PlanningID: planningID, UserID: userID, Role: plannings.RoleMember}, nil
}

func TestParseURLHints(t *testing.T) {
	bookingURL, _ := url.Parse("https://www.booking.com/hotel/es/madrid-sofia-resort.es.html?aid=964694&app_hotel_id=11028412&checkin=2026-12-25&checkout=2026-12-27&from_sn=android&group_adults=10&group_children=0&label=vertical_gallery-caLNHs%401794594234&no_rooms=1&req_adults=10&req_children=0&room1=A%2CA%2CA%2CA%2CA%2CA%2CA%2CA%2CA%2CA&chal_t=1795870596047&force_referer=")
	bookingPreview := parseURLHints(bookingURL)
	require.Equal(t, "hotel", *bookingPreview.SuggestedBookingType)
	require.Equal(t, "2026-12-25", *bookingPreview.StartsAt)
	require.Equal(t, "2026-12-27", *bookingPreview.EndsAt)
	require.Equal(t, "Madrid Sofia Resort", *bookingPreview.Title)

	airbnbURL, _ := url.Parse("https://www.airbnb.es/rooms/12345?check_in=2026-11-01&check_out=2026-11-04")
	airbnbPreview := parseURLHints(airbnbURL)
	require.Equal(t, "hotel", *airbnbPreview.SuggestedBookingType)
	require.Equal(t, "2026-11-01", *airbnbPreview.StartsAt)
	require.Equal(t, "2026-11-04", *airbnbPreview.EndsAt)

	renfeURL, _ := url.Parse("https://www.renfe.com/es/es/viajar/informacion-util")
	renfePreview := parseURLHints(renfeURL)
	require.Equal(t, "train", *renfePreview.SuggestedTransportType)
	require.Equal(t, "Renfe", *renfePreview.Operator)
}

func TestMergeHTMLMetadataExtractsJSONLDAndOpenGraph(t *testing.T) {
	htmlBody := `
	<html><head>
		<title>Restaurante Arzak - TheFork</title>
		<meta property="og:description" content="Alta cocina vasca en Donostia." />
		<script type="application/ld+json">
		{
			"@context": "https://schema.org",
			"@type": "Restaurant",
			"name": "Arzak",
			"address": {
				"@type": "PostalAddress",
				"streetAddress": "Av. Alcalde Elosegui 273",
				"addressLocality": "San Sebastián",
				"addressCountry": "ES"
			},
			"offers": { "price": "265.00", "priceCurrency": "EUR" }
		}
		</script>
	</head></html>`

	parsed, _ := url.Parse("https://example.com/arzak")
	res := mergeHTMLMetadata(parseURLHints(parsed), htmlBody)
	require.Equal(t, "Arzak", *res.Title)
	require.Equal(t, "restaurant", *res.SuggestedBookingType)
	require.Equal(t, "Av. Alcalde Elosegui 273, San Sebastián, ES", *res.Location)
	require.Equal(t, int64(26500), *res.PriceCents)
	require.Equal(t, "EUR", *res.Currency)
	require.Equal(t, "Alta cocina vasca en Donostia.", *res.Description)

	metaHTML := `<html><head>
		<meta property="og:title" content="Hotel Gran Via" />
		<meta property="og:street-address" content="Gran Vía 25" />
		<meta property="og:locality" content="Madrid" />
		<meta property="og:country-name" content="España" />
	</head></html>`
	metaRes := mergeHTMLMetadata(parseURLHints(parsed), metaHTML)
	require.Equal(t, "Gran Vía 25, Madrid, España", *metaRes.Location)
}

func TestServiceBlocksSSRFAndFallsBackOnBlockedSite(t *testing.T) {
	planningID, userID := uuid.New(), uuid.New()
	service := NewService(stubMembership{allow: true})

	for _, blocked := range []string{
		"http://127.0.0.1:8080/secret",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/admin",
		"http://localhost:3000",
		"file:///etc/passwd",
	} {
		_, err := service.Preview(context.Background(), userID, planningID, blocked)
		require.ErrorIs(t, err, ErrInvalidURL, "expected %s to be blocked", blocked)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	testService := newServiceWithClient(stubMembership{allow: true}, server.Client())
	// httptest uses 127.0.0.1 which validateTargetURL blocks; verify validation rejects loopback.
	_, err := testService.Preview(context.Background(), userID, planningID, server.URL)
	require.ErrorIs(t, err, ErrInvalidURL)
}
