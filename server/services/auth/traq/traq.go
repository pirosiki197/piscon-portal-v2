// Package traq は、traQ の OAuth2 / OIDC でユーザーを確かめる認証手段である。
package traq

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/jellydator/ttlcache/v3"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"golang.org/x/oauth2"
)

// ProviderID は、traQ の認証手段の ID である。
const ProviderID = "traq"

type Config struct {
	Issuer       string
	ClientID     string
	ClientSecret string
	AuthURL      string
	TokenURL     string
}

type Provider struct {
	config   *oauth2.Config
	verifier *oidc.IDTokenVerifier
	issuer   string

	// codeVerifiers は、state をキーにして PKCE の code verifier を持つ。
	codeVerifiers *ttlcache.Cache[string, string]
}

// New は、traQ の認証手段を作る。Issuer の discovery を取得するので、traQ に繋がらなければ失敗する。
// redirectURL は、traQ から戻ってくるコールバックの絶対 URL である。
func New(ctx context.Context, config Config, redirectURL string) (*Provider, error) {
	oauth2Config := &oauth2.Config{
		ClientID:     config.ClientID,
		ClientSecret: config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:  config.AuthURL,
			TokenURL: config.TokenURL,
		},
		RedirectURL: redirectURL,
		Scopes:      []string{"openid", "profile"},
	}

	provider, err := oidc.NewProvider(ctx, config.Issuer)
	if err != nil {
		return nil, fmt.Errorf("new oidc provider: %w", err)
	}
	verifier := provider.Verifier(&oidc.Config{ClientID: config.ClientID})

	codeVerifiers := ttlcache.New(
		ttlcache.WithTTL[string, string](15 * time.Minute),
	)
	go codeVerifiers.Start()

	return &Provider{
		config:        oauth2Config,
		verifier:      verifier,
		issuer:        config.Issuer,
		codeVerifiers: codeVerifiers,
	}, nil
}

func (p *Provider) ID() string {
	return ProviderID
}

func (p *Provider) DisplayName() string {
	return "traQ"
}

// AuthURL は、traQ の認可ページの URL を返す。PKCE の code verifier を作り、state に紐付けて 15 分間保持する。
func (p *Provider) AuthURL(state string) string {
	codeVerifier := oauth2.GenerateVerifier()
	p.codeVerifiers.Set(state, codeVerifier, ttlcache.DefaultTTL)

	return p.config.AuthCodeURL(state, oauth2.S256ChallengeOption(codeVerifier))
}

// Complete は、コールバックのクエリの認可コードをトークンに交換し、ID トークンから外部アカウントを得る。
// state に紐付いた code verifier は、成否にかかわらず消す。
func (p *Provider) Complete(ctx context.Context, state string, query url.Values) (domain.Identity, error) {
	codeVerifier, found := p.codeVerifiers.GetAndDelete(state)
	if !found {
		return domain.Identity{}, errors.New("code verifier not found")
	}
	token, err := p.config.Exchange(ctx, query.Get("code"), oauth2.VerifierOption(codeVerifier.Value()))
	if err != nil {
		return domain.Identity{}, fmt.Errorf("exchange token: %w", err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok {
		return domain.Identity{}, errors.New("missing id_token")
	}
	idToken, err := p.verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return domain.Identity{}, fmt.Errorf("verify id_token: %w", err)
	}

	// traQ は "name" claim に traQ ID を入れる。
	// ref: https://github.com/traPtitech/traQ/blob/v3.21.0/service/oidc/userinfo.go#L57
	var claims struct {
		Name string `json:"name"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return domain.Identity{}, fmt.Errorf("parse id_token claims: %w", err)
	}

	iconURL, err := url.JoinPath(p.issuer, "/api/v3/public/icon", claims.Name)
	if err != nil {
		return domain.Identity{}, fmt.Errorf("join icon URL: %w", err)
	}

	return domain.Identity{
		Provider:    ProviderID,
		Subject:     idToken.Subject,
		DefaultName: claims.Name,
		IconURL:     iconURL,
	}, nil
}
