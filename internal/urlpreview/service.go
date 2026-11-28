package urlpreview

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/eleventravel/eleventravel-api/internal/plannings"
	"github.com/google/uuid"
)

const maxResponseBytes = 512 * 1024

type Service struct {
	membership PlanningMembership
	httpClient *http.Client
}

func NewService(membership PlanningMembership) *Service {
	return newServiceWithClient(membership, newSafeHTTPClient())
}

func newServiceWithClient(membership PlanningMembership, client *http.Client) *Service {
	return &Service{membership: membership, httpClient: client}
}

func newSafeHTTPClient() *http.Client {
	dialer := &net.Dialer{Timeout: 4 * time.Second}
	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, ip := range ips {
				if isBlockedIP(ip.IP) {
					return nil, fmt.Errorf("%w: blocked host IP", ErrInvalidURL)
				}
			}
			for _, ip := range ips {
				conn, dialErr := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
				if dialErr == nil {
					return conn, nil
				}
				err = dialErr
			}
			if err == nil {
				return nil, fmt.Errorf("%w: no reachable address", ErrInvalidURL)
			}
			return nil, err
		},
	}
	return &http.Client{
		Timeout:   6 * time.Second,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many redirects")
			}
			if err := validateTargetURL(req.URL); err != nil {
				return err
			}
			return nil
		},
	}
}

func validateTargetURL(parsed *url.URL) error {
	if parsed == nil {
		return ErrInvalidURL
	}
	scheme := strings.ToLower(parsed.Scheme)
	if (scheme != "http" && scheme != "https") || strings.TrimSpace(parsed.Hostname()) == "" {
		return ErrInvalidURL
	}
	if ip := net.ParseIP(parsed.Hostname()); ip != nil && isBlockedIP(ip) {
		return ErrInvalidURL
	}
	if strings.EqualFold(parsed.Hostname(), "localhost") || strings.HasSuffix(strings.ToLower(parsed.Hostname()), ".local") {
		return ErrInvalidURL
	}
	return nil
}

func isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	return ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified()
}

func (s *Service) Preview(ctx context.Context, actor, planning uuid.UUID, rawURL string) (PreviewResponse, error) {
	if _, err := s.membership.GetMember(ctx, planning, actor); err != nil {
		if errors.Is(err, plannings.ErrMemberNotFound) {
			return PreviewResponse{}, ErrForbidden
		}
		return PreviewResponse{}, err
	}

	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" || len([]rune(trimmed)) > 1000 {
		return PreviewResponse{}, ErrInvalidURL
	}
	parsed, err := url.ParseRequestURI(trimmed)
	if err != nil {
		return PreviewResponse{}, ErrInvalidURL
	}
	if err := validateTargetURL(parsed); err != nil {
		return PreviewResponse{}, err
	}

	preview := parseURLHints(parsed)

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return preview, nil
	}
	request.Header.Set("User-Agent", "Mozilla/5.0 (compatible; ElevenTravelBot/1.0)")
	request.Header.Set("Accept", "text/html,application/xhtml+xml")

	resp, err := s.httpClient.Do(request)
	if err != nil {
		return preview, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return preview, nil
	}

	contentType := strings.ToLower(resp.Header.Get("Content-Type"))
	if contentType != "" && !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml") {
		return preview, nil
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return preview, nil
	}
	return mergeHTMLMetadata(preview, string(bodyBytes)), nil
}
