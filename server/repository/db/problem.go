package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/samber/lo"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/repository/db/models"
)

func (r *Repository) GetProblemName(ctx context.Context) (string, error) {
	problem, err := models.Problems.Query().One(ctx, r.executor(ctx))
	if errors.Is(err, sql.ErrNoRows) {
		return "", repository.ErrNotFound
	}
	if err != nil {
		return "", fmt.Errorf("get problem: %w", err)
	}
	return problem.Name, nil
}

func (r *Repository) CreateProblem(ctx context.Context, name string) error {
	// id は 1 に固定してあるので、2 つ目の問題は主キーの重複で失敗する。
	_, err := models.Problems.Insert(&models.ProblemSetter{
		Name: lo.ToPtr(name),
	}).Exec(ctx, r.executor(ctx))
	if err != nil {
		return fmt.Errorf("insert problem: %w", err)
	}
	return nil
}
