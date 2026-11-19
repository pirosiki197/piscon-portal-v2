// Package dev は、名前を入力するだけでログインできる開発用の認証手段である。
// 誰でも好きな名前のユーザーとしてログインできるので、本番では有効にしない。
package dev

import (
	"context"
	"errors"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/traPtitech/piscon-portal-v2/server/domain"
)

// ProviderID は、開発用の認証手段の ID である。
const ProviderID = "dev"

type Provider struct {
	authorizePagePath string
	callbackPath      string
}

// New は、開発用の認証手段を作る。
// authorizePagePath は AuthorizePage を公開するパス、callbackPath はポータルのコールバックのパスである。
func New(authorizePagePath, callbackPath string) *Provider {
	return &Provider{
		authorizePagePath: authorizePagePath,
		callbackPath:      callbackPath,
	}
}

func (p *Provider) ID() string {
	return ProviderID
}

func (p *Provider) DisplayName() string {
	return "dev"
}

// AuthURL は、名前を入力する偽の認可ページの URL を返す。ポータル内のパスなので、ホストを含まない。
func (p *Provider) AuthURL(state string) string {
	return p.authorizePagePath + "?" + url.Values{"state": {state}}.Encode()
}

// Complete は、偽の認可ページで入力された名前を、subject と名前の初期値にした外部アカウントを返す。
// 名前が空なら失敗する。
func (p *Provider) Complete(_ context.Context, _ string, query url.Values) (domain.Identity, error) {
	name := strings.TrimSpace(query.Get("name"))
	if name == "" {
		return domain.Identity{}, errors.New("name is empty")
	}
	return domain.Identity{
		Provider:    ProviderID,
		Subject:     name,
		DefaultName: name,
	}, nil
}

var authorizePage = template.Must(template.New("authorize").Parse(`<!doctype html>
<html lang="ja">
<head>
<meta charset="utf-8">
<title>dev ログイン</title>
</head>
<body>
<h1>dev ログイン</h1>
<p>入力した名前のユーザーとしてログインします。最初にログインしたユーザーが admin になります。</p>
<form method="get" action="{{.CallbackPath}}">
<input type="hidden" name="state" value="{{.State}}">
<input type="text" name="name" required autofocus placeholder="alice">
<button type="submit">ログイン</button>
</form>
</body>
</html>
`))

// AuthorizePage は、名前を入力する偽の認可ページを返す。送信するとコールバックに state と名前を付けて戻る。
func (p *Provider) AuthorizePage() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_ = authorizePage.Execute(w, struct {
			CallbackPath string
			State        string
		}{
			CallbackPath: p.callbackPath,
			State:        r.URL.Query().Get("state"),
		})
	})
}
