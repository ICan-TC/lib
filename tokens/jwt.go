package tokens

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/cristalhq/jwt/v5"
	"github.com/oklog/ulid/v2"
)

type userClaims struct {
	jwt.RegisteredClaims
	Email       string            `json:"email"`
	Username    string            `json:"username"`
	TokenType   string            `json:"token_type"`
	ExtraClaims map[string]string `json:"extra_claims"`
}

type TokenProvider struct {
	secret     string
	accessExp  int
	refreshExp int
}

type TokenProviderArgs struct {
	Secret          string
	AccessTokenTTL  int
	RefreshTokenTTL int
}

func NewTokenProvider(args TokenProviderArgs) (*TokenProvider, error) {
	secret := args.Secret
	if len(secret) == 0 {
		return nil, errors.New("jwt secret is not set, please set it in config under Auth.Secret")
	}
	aTTL := args.AccessTokenTTL
	rTTL := args.RefreshTokenTTL
	if aTTL == 0 {
		return nil, errors.New("invalid config jwt.exp.access: access token expiry must be greater than 0")
	}
	if rTTL == 0 {
		return nil, errors.New("invalid config jwt.exp.refresh: refresh token expiry must be greater than 0")
	}
	return &TokenProvider{
		secret:     secret,
		accessExp:  aTTL,
		refreshExp: rTTL,
	}, nil
}

func (p *TokenProvider) GetAccess(ctx context.Context, sub, username, email, refreshTokenID string) (*jwt.Token, *time.Time, error) {
	return GenerateToken(ctx, TokenGenerationInput{
		Sub:       sub,
		Secret:    p.secret,
		Exp:       p.accessExp,
		TokenType: "access",
		TokenID:   refreshTokenID,
		ExtraClaims: map[string]string{
			"Username": username,
			"Email":    email,
		},
	})
}

func (p *TokenProvider) GetRefresh(ctx context.Context, sub, username, email string) (*jwt.Token, *time.Time, string, error) {
	id := ulid.Make().String()
	token, exp, err := GenerateToken(ctx, TokenGenerationInput{
		Sub: sub, Secret: p.secret, Exp: p.refreshExp,
		TokenType: "refresh",
		TokenID:   id,
		ExtraClaims: map[string]string{
			"Username": username,
			"Email":    email,
		},
	})
	return token, exp, id, err
}

func (p *TokenProvider) ParseAccess(ctx context.Context, tokenString string) (*userClaims, error) {
	return ParseToken(ctx, tokenString, p.secret, "access")
}

func (p *TokenProvider) ParseRefresh(ctx context.Context, tokenString string) (*userClaims, error) {
	return ParseToken(ctx, tokenString, p.secret, "refresh")
}

type TokenGenerationInput struct {
	Sub         string
	Secret      string
	Exp         int
	TokenType   string
	TokenID     string
	ExtraClaims map[string]string
}

func GenerateToken(
	ctx context.Context,
	in TokenGenerationInput,
) (*jwt.Token, *time.Time, error) {
	// Validate required fields
	if len(in.Secret) == 0 {
		return nil, nil, errors.New("secret must not be empty")
	}
	if in.Exp <= 0 {
		return nil, nil, errors.New("expiration time must be greater than zero")
	}
	if len(in.Sub) == 0 {
		return nil, nil, errors.New("subject must not be empty")
	}
	if len(in.TokenType) == 0 {
		return nil, nil, errors.New("token type must not be empty")
	}
	if in.ExtraClaims == nil {
		in.ExtraClaims = make(map[string]string)
	}

	key := []byte(in.Secret)
	signer, err := jwt.NewSignerHS(jwt.HS256, key)
	if err != nil {
		return nil, nil, err
	}
	expiresAt := time.Now().Add(time.Duration(in.Exp) * time.Second)

	// Defensive: avoid panic if keys missing
	username, _ := in.ExtraClaims["Username"]
	email, _ := in.ExtraClaims["Email"]

	claims := &userClaims{
		Username:    username,
		Email:       email,
		TokenType:   in.TokenType,
		ExtraClaims: in.ExtraClaims,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        in.TokenID,
			Subject:   in.Sub,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	builder := jwt.NewBuilder(signer)
	token, err := builder.Build(claims)
	if err != nil {
		return nil, nil, err
	}
	return token, &expiresAt, nil
}

func ParseToken(ctx context.Context,
	tokenString string,
	secret string,
	tokenType string,
) (*userClaims, error) {
	key := []byte(secret)
	verifier, err := jwt.NewVerifierHS(jwt.HS256, key)
	if err != nil {
		return nil, err
	}
	token, err := jwt.ParseNoVerify([]byte(tokenString))
	if err != nil {
		return nil, err
	}
	var claims userClaims
	err = json.Unmarshal(token.Claims(), &claims)
	if err != nil {
		return nil, err
	}

	var ok bool = claims.IsValidAt(time.Now())
	if !ok {
		return nil, errors.New("token expired")
	}
	if claims.TokenType != tokenType {
		return nil, errors.New("invalid token type, expecting " + tokenType)
	}

	err = verifier.Verify(token)
	if err != nil {
		return nil, err
	}

	return &claims, nil
}
