package urlpreview

import (
	"encoding/json"
	"html"
	"math"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

var (
	jsonLDPattern  = regexp.MustCompile(`(?is)<script[^>]*type=["']application/ld\+json["'][^>]*>(.*?)</script>`)
	metaTagPattern = regexp.MustCompile(`(?is)<meta\b([^>]+)>`)
	attrPattern    = regexp.MustCompile(`(?is)([a-zA-Z0-9_:-]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|([^\s>]+))`)
	titlePattern     = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
	dateOnlyRegex    = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	langSuffixRegex  = regexp.MustCompile(`\.[a-z]{2}(?:-[a-z]{2})?$`)
)

func parseURLHints(parsed *url.URL) PreviewResponse {
	host := strings.ToLower(strings.TrimPrefix(parsed.Hostname(), "www."))
	res := PreviewResponse{
		URL:          parsed.String(),
		SourceDomain: host,
	}
	query := parsed.Query()
	path := parsed.Path

	switch {
	case strings.Contains(host, "booking.com"):
		res.SuggestedBookingType = strPtr("hotel")
		if v := cleanDate(query.Get("checkin")); v != nil {
			res.StartsAt = v
		}
		if v := cleanDate(query.Get("checkout")); v != nil {
			res.EndsAt = v
		}
		if slug := extractBookingSlug(path); slug != "" {
			res.Title = strPtr(slug)
		}
	case strings.Contains(host, "airbnb."):
		res.SuggestedBookingType = strPtr("hotel")
		if v := cleanDate(query.Get("check_in")); v != nil {
			res.StartsAt = v
		}
		if v := cleanDate(query.Get("check_out")); v != nil {
			res.EndsAt = v
		}
	case strings.Contains(host, "maps.google.") || strings.Contains(host, "maps.app.goo.gl") || (strings.Contains(host, "google.") && strings.HasPrefix(path, "/maps")):
		if name := extractGoogleMapsPlace(path); name != "" {
			res.Title = strPtr(name)
			res.Location = strPtr(name)
		}
	case strings.Contains(host, "thefork.") || strings.Contains(host, "eltenedor."):
		res.SuggestedBookingType = strPtr("restaurant")
	case strings.Contains(host, "getyourguide.") || strings.Contains(host, "civitatis.") || strings.Contains(host, "viator.") || strings.Contains(host, "tiqets."):
		res.SuggestedBookingType = strPtr("activity")
	case strings.Contains(host, "tripadvisor."):
		if strings.Contains(path, "/Hotel_Review") {
			res.SuggestedBookingType = strPtr("hotel")
		} else if strings.Contains(path, "/Restaurant_Review") {
			res.SuggestedBookingType = strPtr("restaurant")
		} else if strings.Contains(path, "/Attraction_Review") {
			res.SuggestedBookingType = strPtr("attraction")
		}
	case strings.Contains(host, "renfe.com"):
		res.SuggestedTransportType = strPtr("train")
		res.Operator = strPtr("Renfe")
	case strings.Contains(host, "iryo.eu"):
		res.SuggestedTransportType = strPtr("train")
		res.Operator = strPtr("Iryo")
	case strings.Contains(host, "ouigo.com"):
		res.SuggestedTransportType = strPtr("train")
		res.Operator = strPtr("Ouigo")
	case strings.Contains(host, "omio.") || strings.Contains(host, "trainline.") || strings.Contains(host, "thetrainline.com"):
		res.SuggestedTransportType = strPtr("train")
	case strings.Contains(host, "alsa."):
		res.SuggestedTransportType = strPtr("bus")
		res.Operator = strPtr("Alsa")
	case strings.Contains(host, "flixbus."):
		res.SuggestedTransportType = strPtr("bus")
		res.Operator = strPtr("FlixBus")
	case strings.Contains(host, "rentalcars.com"):
		res.SuggestedTransportType = strPtr("rental_car")
	case strings.Contains(host, "sixt."):
		res.SuggestedTransportType = strPtr("rental_car")
		res.Operator = strPtr("Sixt")
	case strings.Contains(host, "hertz."):
		res.SuggestedTransportType = strPtr("rental_car")
		res.Operator = strPtr("Hertz")
	case strings.Contains(host, "avis."):
		res.SuggestedTransportType = strPtr("rental_car")
		res.Operator = strPtr("Avis")
	case strings.Contains(host, "europcar."):
		res.SuggestedTransportType = strPtr("rental_car")
		res.Operator = strPtr("Europcar")
	case strings.Contains(host, "balearia."):
		res.SuggestedTransportType = strPtr("ferry")
		res.Operator = strPtr("Baleària")
	case strings.Contains(host, "directferries."):
		res.SuggestedTransportType = strPtr("ferry")
	}

	return res
}

func mergeHTMLMetadata(base PreviewResponse, body string) PreviewResponse {
	for _, match := range jsonLDPattern.FindAllStringSubmatch(body, 10) {
		if len(match) < 2 {
			continue
		}
		var payload any
		if err := json.Unmarshal([]byte(strings.TrimSpace(match[1])), &payload); err != nil {
			continue
		}
		applyJSONLD(&base, payload)
	}

	metas := extractMetaTags(body)
	if base.Title == nil {
		if title := firstNonEmpty(metas["og:title"], metas["twitter:title"], extractHTMLTitle(body)); title != "" {
			cleaned := cleanBrandSuffix(title)
			if cleaned != "" {
				base.Title = &cleaned
			}
		}
	}
	if base.Description == nil {
		if desc := firstNonEmpty(metas["og:description"], metas["description"]); desc != "" {
			truncated := truncateRunes(desc, 280)
			base.Description = &truncated
		}
	}
	if base.Location == nil {
		if loc := extractMetaLocation(metas); loc != "" {
			base.Location = &loc
		}
	}
	return base
}

func extractMetaLocation(metas map[string]string) string {
	parts := []string{}
	for _, key := range []string{"og:street-address", "business:contact_data:street_address", "og:locality", "business:contact_data:locality", "og:region", "og:country-name", "business:contact_data:country_name"} {
		if val := strings.TrimSpace(metas[key]); val != "" {
			duplicate := false
			for _, existing := range parts {
				if strings.EqualFold(existing, val) {
					duplicate = true
					break
				}
			}
			if !duplicate {
				parts = append(parts, val)
			}
		}
	}
	if len(parts) > 0 {
		return truncateRunes(strings.Join(parts, ", "), 140)
	}
	if place := strings.TrimSpace(metas["geo.placename"]); place != "" {
		return truncateRunes(place, 140)
	}
	return ""
}

func applyJSONLD(res *PreviewResponse, node any) {
	switch typed := node.(type) {
	case []any:
		for _, item := range typed {
			applyJSONLD(res, item)
		}
	case map[string]any:
		if graph, ok := typed["@graph"]; ok {
			applyJSONLD(res, graph)
		}
		if typ := jsonLDType(typed["@type"]); typ != "" {
			if mapped := mapSchemaType(typ); mapped != "" && res.SuggestedBookingType == nil {
				res.SuggestedBookingType = strPtr(mapped)
			}
			if name, ok := typed["name"].(string); ok && strings.TrimSpace(name) != "" {
				cleaned := cleanBrandSuffix(html.UnescapeString(strings.TrimSpace(name)))
				if cleaned != "" {
					res.Title = &cleaned
				}
			}
			if desc, ok := typed["description"].(string); ok && strings.TrimSpace(desc) != "" && res.Description == nil {
				truncated := truncateRunes(html.UnescapeString(strings.TrimSpace(desc)), 280)
				res.Description = &truncated
			}
			if res.Location == nil {
				if addr := extractAddress(typed["address"]); addr != "" {
					res.Location = &addr
				}
			}
			if res.StartsAt == nil {
				if s := firstNonEmpty(stringField(typed, "checkinTime"), stringField(typed, "checkinDate"), stringField(typed, "startDate")); s != "" {
					res.StartsAt = &s
				}
			}
			if res.EndsAt == nil {
				if e := firstNonEmpty(stringField(typed, "checkoutTime"), stringField(typed, "checkoutDate"), stringField(typed, "endDate")); e != "" {
					res.EndsAt = &e
				}
			}
			if res.PriceCents == nil {
				extractOfferPrice(res, typed["offers"])
			}
		}
	}
}

func jsonLDType(raw any) string {
	switch v := raw.(type) {
	case string:
		return v
	case []any:
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func mapSchemaType(schemaType string) string {
	switch strings.ToLower(strings.TrimSpace(schemaType)) {
	case "hotel", "lodgingbusiness", "bedandbreakfast", "hostel", "resort", "motel", "vacationrental", "apartment":
		return "hotel"
	case "restaurant", "foodestablishment", "cafeorcoffeeshop", "barorpub", "bakery":
		return "restaurant"
	case "touristattraction", "museum", "landmarksorhistoricalbuildings", "park", "amusementpark":
		return "attraction"
	case "event", "tour", "trip":
		return "activity"
	default:
		return ""
	}
}

func extractAddress(raw any) string {
	switch v := raw.(type) {
	case string:
		return strings.TrimSpace(v)
	case map[string]any:
		parts := []string{}
		for _, key := range []string{"streetAddress", "addressLocality", "addressRegion", "addressCountry"} {
			if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
				parts = append(parts, strings.TrimSpace(s))
			}
		}
		return strings.Join(parts, ", ")
	}
	return ""
}

func extractOfferPrice(res *PreviewResponse, raw any) {
	var offer map[string]any
	switch v := raw.(type) {
	case map[string]any:
		offer = v
	case []any:
		if len(v) > 0 {
			offer, _ = v[0].(map[string]any)
		}
	}
	if offer == nil {
		return
	}
	currency := strings.ToUpper(strings.TrimSpace(stringField(offer, "priceCurrency")))
	if currency != "" && currency != "EUR" {
		return
	}
	var price float64
	switch p := offer["price"].(type) {
	case float64:
		price = p
	case string:
		parsed, err := strconv.ParseFloat(strings.ReplaceAll(strings.TrimSpace(p), ",", "."), 64)
		if err != nil {
			return
		}
		price = parsed
	default:
		return
	}
	if price > 0 {
		cents := int64(math.Round(price * 100))
		res.PriceCents = &cents
		if currency != "" {
			res.Currency = &currency
		}
	}
}

func extractMetaTags(body string) map[string]string {
	result := map[string]string{}
	for _, match := range metaTagPattern.FindAllStringSubmatch(body, 60) {
		if len(match) < 2 {
			continue
		}
		attrs := map[string]string{}
		for _, attr := range attrPattern.FindAllStringSubmatch(match[1], -1) {
			key := strings.ToLower(attr[1])
			val := firstNonEmpty(attr[2], attr[3], attr[4])
			attrs[key] = html.UnescapeString(strings.TrimSpace(val))
		}
		name := strings.ToLower(firstNonEmpty(attrs["property"], attrs["name"]))
		content := attrs["content"]
		if name != "" && content != "" {
			if _, exists := result[name]; !exists {
				result[name] = content
			}
		}
	}
	return result
}

func extractHTMLTitle(body string) string {
	match := titlePattern.FindStringSubmatch(body)
	if len(match) < 2 {
		return ""
	}
	return html.UnescapeString(strings.TrimSpace(match[1]))
}

func extractBookingSlug(path string) string {
	idx := strings.Index(path, "/hotel/")
	if idx < 0 {
		return ""
	}
	rest := strings.TrimPrefix(path[idx+len("/hotel/"):], "/")
	parts := strings.Split(rest, "/")
	if len(parts) < 2 {
		return ""
	}
	slug := strings.TrimSuffix(parts[1], ".html")
	slug = langSuffixRegex.ReplaceAllString(slug, "")
	words := strings.Fields(strings.ReplaceAll(slug, "-", " "))
	for i, w := range words {
		if len(w) > 0 {
			words[i] = strings.ToUpper(w[:1]) + w[1:]
		}
	}
	return strings.Join(words, " ")
}

func extractGoogleMapsPlace(path string) string {
	idx := strings.Index(path, "/maps/place/")
	if idx < 0 {
		return ""
	}
	rest := path[idx+len("/maps/place/"):]
	segment := strings.SplitN(rest, "/", 2)[0]
	decoded, err := url.PathUnescape(strings.ReplaceAll(segment, "+", " "))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(decoded)
}

func cleanBrandSuffix(title string) string {
	for _, sep := range []string{" - Booking.com", " | Booking.com", " - Airbnb", " | Airbnb", " - Tripadvisor", " | Tripadvisor", " - Google Maps", " - TheFork", " | TheFork"} {
		if idx := strings.Index(strings.ToLower(title), strings.ToLower(sep)); idx > 0 {
			title = title[:idx]
		}
	}
	return truncateRunes(strings.TrimSpace(title), 140)
}

func cleanDate(value string) *string {
	trimmed := strings.TrimSpace(value)
	if !dateOnlyRegex.MatchString(trimmed) {
		return nil
	}
	return &trimmed
}

func stringField(m map[string]any, key string) string {
	if s, ok := m[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateRunes(value string, max int) string {
	runes := []rune(value)
	if len(runes) <= max {
		return value
	}
	return strings.TrimSpace(string(runes[:max]))
}

func strPtr(value string) *string {
	return &value
}
