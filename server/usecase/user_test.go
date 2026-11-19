package usecase_test

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	repomock "github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	"github.com/traPtitech/piscon-portal-v2/server/utils/testutil"
	"go.uber.org/mock/gomock"
)

func TestGetUsers(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	mockRepo := repomock.NewMockRepository(ctrl)
	usecase := usecase.NewUserUseCase(mockRepo)

	users := []domain.User{
		{ID: uuid.New(), Name: "user1", TeamID: uuid.NullUUID{UUID: uuid.New(), Valid: true}},
		{ID: uuid.New(), Name: "user2"},
		{ID: uuid.New(), Name: "user3"},
	}

	mockRepo.EXPECT().GetUsers(gomock.Any()).Return(users, nil)

	got, err := usecase.GetUsers(t.Context())
	assert.NoError(t, err)

	testutil.CompareUsers(t, users, got)
}

func TestLogin(t *testing.T) {
	t.Parallel()

	identity := domain.Identity{Provider: "dev", Subject: "alice", DefaultName: "alice"}
	existingUserID := uuid.New()

	tests := map[string]struct {
		setup     func(repo *repomock.MockRepository)
		wantAdmin bool
		// wantUserID が Nil でなければ、返すユーザーの ID を比べる
		wantUserID     uuid.UUID
		wantErr        bool
		wantUseCaseErr bool
	}{
		"既存のユーザーならそのまま返す": {
			setup: func(repo *repomock.MockRepository) {
				repo.EXPECT().FindUserByIdentity(gomock.Any(), "dev", "alice").
					Return(domain.User{ID: existingUserID, Name: "alice"}, nil)
			},
			wantUserID: existingUserID,
		},
		"最初のユーザーなら admin にする": {
			setup: func(repo *repomock.MockRepository) {
				repo.EXPECT().FindUserByIdentity(gomock.Any(), "dev", "alice").Return(domain.User{}, repository.ErrNotFound)
				repo.EXPECT().CountUsers(gomock.Any()).Return(int64(0), nil)
				repo.EXPECT().CreateUser(gomock.Any(), gomock.Cond(func(u domain.User) bool {
					return u.IsAdmin && u.Name == "alice" && u.Provider == "dev" && u.Subject == "alice"
				})).Return(nil)
			},
			wantAdmin: true,
		},
		"2 人目以降のユーザーは admin にしない": {
			setup: func(repo *repomock.MockRepository) {
				repo.EXPECT().FindUserByIdentity(gomock.Any(), "dev", "alice").Return(domain.User{}, repository.ErrNotFound)
				repo.EXPECT().CountUsers(gomock.Any()).Return(int64(1), nil)
				repo.EXPECT().CreateUser(gomock.Any(), gomock.Cond(func(u domain.User) bool { return !u.IsAdmin })).Return(nil)
			},
		},
		"名前が使われていれば UseCaseError": {
			setup: func(repo *repomock.MockRepository) {
				repo.EXPECT().FindUserByIdentity(gomock.Any(), "dev", "alice").Return(domain.User{}, repository.ErrNotFound)
				repo.EXPECT().CountUsers(gomock.Any()).Return(int64(1), nil)
				repo.EXPECT().CreateUser(gomock.Any(), gomock.Any()).Return(repository.ErrAlreadyExists)
			},
			wantErr:        true,
			wantUseCaseErr: true,
		},
		"探すのに失敗したらエラー": {
			setup: func(repo *repomock.MockRepository) {
				repo.EXPECT().FindUserByIdentity(gomock.Any(), "dev", "alice").Return(domain.User{}, errors.New("db error"))
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			mockRepo := repomock.NewMockRepository(ctrl)
			mockRepo.EXPECT().Transaction(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(context.Context) error) error { return f(ctx) })
			tt.setup(mockRepo)

			got, err := usecase.NewUserUseCase(mockRepo).Login(t.Context(), identity)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Equal(t, tt.wantUseCaseErr, usecase.IsUseCaseError(err))
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.wantAdmin, got.IsAdmin)
			if tt.wantUserID != uuid.Nil {
				assert.Equal(t, tt.wantUserID, got.ID)
			}
		})
	}
}
