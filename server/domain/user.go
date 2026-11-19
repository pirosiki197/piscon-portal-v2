package domain

import "github.com/google/uuid"

type User struct {
	ID   uuid.UUID
	Name string
	// Provider と Subject は、ユーザーを作ったときの外部アカウントを指す。作った後は変わらない。
	Provider string
	Subject  string
	// IconURL は、アイコン画像の URL である。空なら既定のアイコンを使う。
	IconURL string
	IsAdmin bool

	TeamID uuid.NullUUID
}

// NewUser は、外部アカウントから新しいユーザーを作る。名前とアイコンの URL は外部アカウントの値を初期値にし、ID は新しく採番する。
func NewUser(identity Identity) User {
	return User{
		ID:       uuid.New(),
		Name:     identity.DefaultName,
		Provider: identity.Provider,
		Subject:  identity.Subject,
		IconURL:  identity.IconURL,
	}
}

// Identity は、認証手段が確かめた外部アカウントである。
type Identity struct {
	// Provider は、認証手段の ID である。users に保存するので、一度使い始めたら変えない。
	Provider string
	// Subject は、認証手段の中でユーザーを一意に指す値である。外部アカウントの名前が変わっても変わらない。
	Subject string
	// DefaultName は、ユーザーを作るときの名前の初期値である。
	DefaultName string
	// IconURL は、ユーザーを作るときのアイコン URL の初期値である。空なら既定のアイコンを使う。
	IconURL string
}
