package repository

import "context"

type ProblemRepository interface {
	// GetProblemName は、DB に記録した問題の名前を返す。
	// まだ記録していなければ ErrNotFound を返す。
	GetProblemName(ctx context.Context) (string, error)
	// CreateProblem は、問題の名前を DB に記録する。DB に記録できる問題は 1 つだけで、
	// すでに記録していればエラーを返す。
	CreateProblem(ctx context.Context, name string) error
}
