package supabasejwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	Email string `json:"email"`
	Role  string `json:"role"`
	jwt.RegisteredClaims
}

type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (Claims, error)
}

type Validator struct {
	secret        []byte
	issuer        string
	audience      string
	jwksURLs      []string
	issuerAliases map[string]struct{}

	httpClient *http.Client

	mu          sync.RWMutex
	publicKeys  map[string]any
	keysExpires time.Time
}

func NewValidator(secret string, issuer string, audience string) (*Validator, error) {
	trimmedSecret := strings.TrimSpace(secret)
	trimmedIssuer := strings.TrimSpace(issuer)

	if trimmedSecret == "" && trimmedIssuer == "" {
		return nil, errors.New("SUPABASE_JWT_SECRET or SUPABASE_JWT_ISSUER is required")
	}

	trimmedIssuer = strings.TrimSuffix(trimmedIssuer, "/")
	issuerAliases := buildIssuerAliases(trimmedIssuer)
	jwksURLs := buildJWKSURLs(trimmedIssuer)

	return &Validator{
		secret:        []byte(trimmedSecret),
		issuer:        trimmedIssuer,
		audience:      audience,
		jwksURLs:      jwksURLs,
		issuerAliases: issuerAliases,
		httpClient:    &http.Client{Timeout: 5 * time.Second},
		publicKeys:    make(map[string]any),
	}, nil
}

func (v *Validator) ValidateToken(ctx context.Context, token string) (Claims, error) {
	parsedToken, err := jwt.ParseWithClaims(token, &Claims{}, func(parsedToken *jwt.Token) (any, error) {
		switch parsedToken.Method.(type) {
		case *jwt.SigningMethodHMAC:
			if len(v.secret) == 0 {
				return nil, errors.New("hmac jwt is not enabled: SUPABASE_JWT_SECRET is missing")
			}
			return v.secret, nil
		case *jwt.SigningMethodECDSA, *jwt.SigningMethodRSA:
			return v.findPublicKey(ctx, parsedToken)
		default:
			return nil, fmt.Errorf("unexpected signing method %s", parsedToken.Method.Alg())
		}
	},
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
			jwt.SigningMethodES256.Alg(),
			jwt.SigningMethodRS256.Alg(),
		}),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil {
		return Claims{}, err
	}

	claims, ok := parsedToken.Claims.(*Claims)
	if !ok || !parsedToken.Valid {
		return Claims{}, errors.New("token claims are invalid")
	}

	if v.issuer != "" {
		if _, ok := v.issuerAliases[claims.Issuer]; !ok {
			return Claims{}, errors.New("token issuer does not match")
		}
	}

	if v.audience != "" {
		audienceMatches := false
		for _, audience := range claims.Audience {
			if audience == v.audience {
				audienceMatches = true
				break
			}
		}

		if !audienceMatches {
			return Claims{}, errors.New("token audience does not match")
		}
	}

	return *claims, nil
}

func (v *Validator) findPublicKey(ctx context.Context, token *jwt.Token) (any, error) {
	kid, _ := token.Header["kid"].(string)
	if strings.TrimSpace(kid) == "" {
		return nil, errors.New("jwt kid header is missing")
	}

	if key := v.getCachedKey(kid); key != nil {
		return key, nil
	}

	if len(v.jwksURLs) == 0 {
		return nil, errors.New("jwks url is missing; set SUPABASE_JWT_ISSUER")
	}

	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}

	if key := v.getCachedKey(kid); key != nil {
		return key, nil
	}

	return nil, fmt.Errorf("public key not found for kid %s", kid)
}

func (v *Validator) getCachedKey(kid string) any {
	v.mu.RLock()
	defer v.mu.RUnlock()

	if time.Now().After(v.keysExpires) {
		return nil
	}

	return v.publicKeys[kid]
}

func (v *Validator) refreshKeys(ctx context.Context) error {
	var payload jwksPayload
	var fetchErr error

	for _, jwksURL := range v.jwksURLs {
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, jwksURL, nil)
		if err != nil {
			fetchErr = err
			continue
		}

		response, err := v.httpClient.Do(request)
		if err != nil {
			fetchErr = fmt.Errorf("fetch jwks (%s): %w", jwksURL, err)
			continue
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			fetchErr = fmt.Errorf("fetch jwks (%s): unexpected status %d", jwksURL, response.StatusCode)
			continue
		}

		decodeErr := json.NewDecoder(response.Body).Decode(&payload)
		response.Body.Close()
		if decodeErr != nil {
			fetchErr = fmt.Errorf("decode jwks (%s): %w", jwksURL, decodeErr)
			continue
		}

		fetchErr = nil
		break
	}

	if fetchErr != nil {
		return fetchErr
	}

	converted := make(map[string]any)
	for _, key := range payload.Keys {
		parsedKey, parseErr := parseJWKKey(key)
		if parseErr != nil {
			continue
		}
		converted[key.KID] = parsedKey
	}

	v.mu.Lock()
	v.publicKeys = converted
	v.keysExpires = time.Now().Add(10 * time.Minute)
	v.mu.Unlock()

	return nil
}

func buildIssuerAliases(issuer string) map[string]struct{} {
	aliases := make(map[string]struct{})
	if issuer == "" {
		return aliases
	}

	normalized := strings.TrimSuffix(strings.TrimSpace(issuer), "/")
	aliases[normalized] = struct{}{}

	if strings.HasSuffix(normalized, "/auth/v1") {
		base := strings.TrimSuffix(normalized, "/auth/v1")
		if base != "" {
			aliases[base] = struct{}{}
		}
	} else {
		aliases[normalized+"/auth/v1"] = struct{}{}
	}

	return aliases
}

func buildJWKSURLs(issuer string) []string {
	if issuer == "" {
		return nil
	}

	normalized := strings.TrimSuffix(strings.TrimSpace(issuer), "/")
	candidates := make([]string, 0, 3)
	push := func(url string) {
		for _, existing := range candidates {
			if existing == url {
				return
			}
		}
		candidates = append(candidates, url)
	}

	if strings.HasSuffix(normalized, "/auth/v1") {
		base := strings.TrimSuffix(normalized, "/auth/v1")
		push(normalized + "/.well-known/jwks.json")
		if base != "" {
			push(base + "/auth/v1/.well-known/jwks.json")
			push(base + "/.well-known/jwks.json")
		}
		return candidates
	}

	push(normalized + "/auth/v1/.well-known/jwks.json")
	push(normalized + "/.well-known/jwks.json")

	return candidates
}

type jwksPayload struct {
	Keys []jwkKey `json:"keys"`
}

type jwkKey struct {
	KTY string `json:"kty"`
	KID string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
	CRV string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func parseJWKKey(key jwkKey) (any, error) {
	if strings.TrimSpace(key.KID) == "" {
		return nil, errors.New("missing kid")
	}

	switch key.KTY {
	case "EC":
		if key.CRV != "P-256" {
			return nil, fmt.Errorf("unsupported ec curve %s", key.CRV)
		}

		x, err := decodeBase64URLInt(key.X)
		if err != nil {
			return nil, err
		}
		y, err := decodeBase64URLInt(key.Y)
		if err != nil {
			return nil, err
		}

		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
	case "RSA":
		n, err := decodeBase64URLInt(key.N)
		if err != nil {
			return nil, err
		}
		e, err := decodeBase64URLInt(key.E)
		if err != nil {
			return nil, err
		}

		return &rsa.PublicKey{N: n, E: int(e.Int64())}, nil
	default:
		return nil, fmt.Errorf("unsupported jwk kty %s", key.KTY)
	}
}

func decodeBase64URLInt(value string) (*big.Int, error) {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return nil, err
	}

	if len(raw) == 0 {
		return nil, errors.New("empty integer payload")
	}

	return new(big.Int).SetBytes(raw), nil
}
