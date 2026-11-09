package usecase_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	"go.uber.org/mock/gomock"
)

func TestRegisterProblem(t *testing.T) {
	t.Parallel()

	type getResult struct {
		name string
		err  error
	}

	testCases := map[string]struct {
		get            getResult
		expectCreate   bool
		createErr      error
		expectDocument bool
		err            error
	}{
		"初めて起動したときは問題を記録してマニュアルを取り込む": {
			get:            getResult{err: repository.ErrNotFound},
			expectCreate:   true,
			expectDocument: true,
		},
		"同じ問題なら何もしない": {
			get: getResult{name: "sample-fib"},
		},
		"別の問題なら起動させない": {
			get: getResult{name: "isucon13"},
			err: usecase.ErrProblemMismatch,
		},
		"問題の取得に失敗する": {
			get: getResult{err: assert.AnError},
			err: assert.AnError,
		},
		"問題の記録に失敗したらマニュアルを取り込まない": {
			get:          getResult{err: repository.ErrNotFound},
			expectCreate: true,
			createErr:    assert.AnError,
			err:          assert.AnError,
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := mock.NewMockRepository(ctrl)
			uc := usecase.NewProblemUseCase(repo)

			repo.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, f func(ctx context.Context) error) error { return f(ctx) },
			)
			repo.EXPECT().GetProblemName(gomock.Any()).Return(tc.get.name, tc.get.err)
			if tc.expectCreate {
				repo.EXPECT().CreateProblem(gomock.Any(), "sample-fib").Return(tc.createErr)
			}
			if tc.expectDocument {
				repo.EXPECT().CreateDocument(gomock.Any(), gomock.Any(), "# manual").Return(domain.Document{}, nil)
			}

			err := uc.RegisterProblem(context.Background(), "sample-fib", "# manual")

			if tc.err != nil {
				assert.ErrorIs(t, err, tc.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
