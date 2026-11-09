package usecase

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
)

// ErrProblemMismatch は、DB に記録した問題と、デプロイされた問題バンドルの問題が違うことを表す。
var ErrProblemMismatch = errors.New("problem mismatch")

type ProblemUseCase interface {
	// RegisterProblem は、DB にまだ問題を記録していなければ、問題の名前を記録し、
	// マニュアルをドキュメントとして取り込む。
	// 記録済みの名前が name と同じなら何もしない。違えば ErrProblemMismatch を返す。
	RegisterProblem(ctx context.Context, name string, manual string) error
}

type problemUseCaseImpl struct {
	repo repository.Repository
}

func NewProblemUseCase(repo repository.Repository) ProblemUseCase {
	return &problemUseCaseImpl{repo: repo}
}

func (u *problemUseCaseImpl) RegisterProblem(ctx context.Context, name string, manual string) error {
	return u.repo.Transaction(ctx, func(ctx context.Context) error {
		registered, err := u.repo.GetProblemName(ctx)
		if err == nil {
			if registered != name {
				return fmt.Errorf("%w: the database is for %q but the bundle is %q; reset the database to switch problems",
					ErrProblemMismatch, registered, name)
			}
			return nil
		}
		if !errors.Is(err, repository.ErrNotFound) {
			return fmt.Errorf("get problem name: %w", err)
		}

		if err := u.repo.CreateProblem(ctx, name); err != nil {
			return fmt.Errorf("create problem: %w", err)
		}
		if _, err := u.repo.CreateDocument(ctx, uuid.New(), manual); err != nil {
			return fmt.Errorf("create document from manual: %w", err)
		}
		return nil
	})
}
