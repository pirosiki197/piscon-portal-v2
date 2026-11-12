package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	"github.com/traPtitech/piscon-portal-v2/server/repository/mock"
	instanceMock "github.com/traPtitech/piscon-portal-v2/server/services/instance/mock"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
	"github.com/traPtitech/piscon-portal-v2/server/utils/ptr"
	"github.com/traPtitech/piscon-portal-v2/server/utils/testutil"
	"go.uber.org/mock/gomock"
)

func TestCreateBenchmark(t *testing.T) {
	t.Parallel()

	userID := uuid.New()
	teamID := uuid.New()
	envID := uuid.New()
	instanceID := uuid.New()

	tests := map[string]struct {
		instanceTeamID uuid.UUID
		instanceName   string
		envStatus      domain.EnvironmentStatus
		instanceStatus domain.InstanceStatus
		benchTarget    string
		queued         []domain.Benchmark
		expectError    bool
	}{
		"success": {
			envStatus:      domain.EnvironmentStatusReady,
			instanceStatus: domain.InstanceStatusRunning,
		},
		"success: target is the fixed server": {
			instanceName:   "Router",
			envStatus:      domain.EnvironmentStatusReady,
			instanceStatus: domain.InstanceStatusRunning,
			benchTarget:    "Router",
		},
		"failure: target is not the fixed server": {
			instanceName:   "Webapp",
			envStatus:      domain.EnvironmentStatusReady,
			instanceStatus: domain.InstanceStatusRunning,
			benchTarget:    "Router",
			expectError:    true,
		},
		"failure: environment is deleting": {
			envStatus:      domain.EnvironmentStatusDeleting,
			instanceStatus: domain.InstanceStatusRunning,
			expectError:    true,
		},
		"failure: instance is not running": {
			envStatus:      domain.EnvironmentStatusReady,
			instanceStatus: domain.InstanceStatusStopped,
			expectError:    true,
		},
		"failure: user's teamID does not match instance's teamID": {
			instanceTeamID: uuid.New(),
			envStatus:      domain.EnvironmentStatusReady,
			instanceStatus: domain.InstanceStatusRunning,
			expectError:    true,
		},
		"failure: benchmark already queued": {
			envStatus:      domain.EnvironmentStatusReady,
			instanceStatus: domain.InstanceStatusRunning,
			queued:         []domain.Benchmark{{ID: uuid.New(), Status: domain.BenchmarkStatusWaiting}},
			expectError:    true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			mockRepo := mock.NewMockRepository(ctrl)
			mockEnvManager := instanceMock.NewMockEnvironmentManager(ctrl)
			mockManager := instanceMock.NewMockManager(ctrl)
			useCase := usecase.NewBenchmarkUseCase(mockRepo, mockEnvManager, mockManager, tt.benchTarget)

			instanceTeamID := teamID
			if tt.instanceTeamID != uuid.Nil {
				instanceTeamID = tt.instanceTeamID
			}
			instance := domain.Instance{
				ID:            instanceID,
				TeamID:        instanceTeamID,
				EnvironmentID: envID,
				Name:          tt.instanceName,
				Infra:         domain.InfraInstance{ProviderInstanceID: "i-1"},
			}

			mockRepo.EXPECT().Transaction(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(context.Context) error) error { return f(ctx) })
			mockRepo.EXPECT().FindUser(gomock.Any(), userID).
				Return(domain.User{ID: userID, TeamID: uuid.NullUUID{Valid: true, UUID: teamID}}, nil)
			mockRepo.EXPECT().FindInstance(gomock.Any(), instanceID).Return(instance, nil)
			mockRepo.EXPECT().FindEnvironment(gomock.Any(), envID).Return(domain.Environment{
				ID:                    envID,
				TeamID:                instanceTeamID,
				ProviderEnvironmentID: "piscon-env",
				Instances:             []domain.Instance{instance},
			}, nil)
			mockEnvManager.EXPECT().Get(gomock.Any(), "piscon-env").
				Return(domain.InfraEnvironment{Status: tt.envStatus}, nil)
			mockManager.EXPECT().GetByIDs(gomock.Any(), []string{"i-1"}).Return([]domain.InfraInstance{{
				ProviderInstanceID: "i-1",
				Status:             tt.instanceStatus,
				PrivateIP:          ptr.Of("10.0.0.1"),
			}}, nil)
			mockRepo.EXPECT().GetBenchmarks(gomock.Any(), gomock.Any()).Return(tt.queued, nil).MaxTimes(1)
			if !tt.expectError {
				mockRepo.EXPECT().CreateBenchmark(gomock.Any(), gomock.Any()).Return(nil)
			}

			_, err := useCase.CreateBenchmark(t.Context(), instanceID, userID)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateBenchmark_InstanceNotFound(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)
	mockRepo := mock.NewMockRepository(ctrl)
	useCase := usecase.NewBenchmarkUseCase(mockRepo, instanceMock.NewMockEnvironmentManager(ctrl), instanceMock.NewMockManager(ctrl), "")

	userID := uuid.New()
	instanceID := uuid.New()
	mockRepo.EXPECT().Transaction(gomock.Any(), gomock.Any()).
		DoAndReturn(func(ctx context.Context, f func(context.Context) error) error { return f(ctx) })
	mockRepo.EXPECT().FindUser(gomock.Any(), userID).
		Return(domain.User{ID: userID, TeamID: uuid.NullUUID{Valid: true, UUID: uuid.New()}}, nil)
	mockRepo.EXPECT().FindInstance(gomock.Any(), instanceID).Return(domain.Instance{}, repository.ErrNotFound)

	_, err := useCase.CreateBenchmark(t.Context(), instanceID, userID)
	assert.Error(t, err)
}

func TestSaveBenchmarkProgress(t *testing.T) {
	t.Parallel()

	type (
		FindBenchmarkResult struct {
			benchmark domain.Benchmark
			err       error
		}
		UpdateBenchmarkResult struct {
			err error
		}
		UpdateBenchmarkLogResult struct {
			err error
		}
	)

	benchLog := domain.BenchmarkLog{
		UserLog:  "user log",
		AdminLog: "admin log",
	}

	testCases := map[string]struct {
		benchmarkID              uuid.UUID
		benchLog                 domain.BenchmarkLog
		score                    int64
		startedAt                time.Time
		FindBenchmarkResult      *FindBenchmarkResult
		UpdateBenchmarkResult    *UpdateBenchmarkResult
		UpdateBenchmarkLogResult *UpdateBenchmarkLogResult
		isErr                    bool
		err                      error
		isUseCaseError           bool
	}{
		"FindBenchmarkでErrNotFoundなのでErrNotFound": {
			benchmarkID:         uuid.New(),
			benchLog:            benchLog,
			score:               0,
			startedAt:           time.Now(),
			FindBenchmarkResult: &FindBenchmarkResult{err: repository.ErrNotFound},
			isErr:               true,
			err:                 usecase.ErrNotFound,
		},
		"FindBenchmarkでErrNotFound以外のエラーが返ってきたのでエラー": {
			benchmarkID:         uuid.New(),
			benchLog:            benchLog,
			score:               0,
			startedAt:           time.Now(),
			FindBenchmarkResult: &FindBenchmarkResult{err: assert.AnError},
			isErr:               true,
			err:                 assert.AnError,
		},
		"benchmarkがrunningでないのでUseCaseError": {
			benchmarkID:         uuid.New(),
			benchLog:            benchLog,
			score:               0,
			startedAt:           time.Now(),
			FindBenchmarkResult: &FindBenchmarkResult{benchmark: domain.Benchmark{Status: domain.BenchmarkStatusWaiting}},
			isErr:               true,
			isUseCaseError:      true,
		},
		"UpdateBenchmarkでエラーなのでエラー": {
			benchmarkID:           uuid.New(),
			benchLog:              benchLog,
			score:                 0,
			startedAt:             time.Now(),
			FindBenchmarkResult:   &FindBenchmarkResult{benchmark: domain.Benchmark{Status: domain.BenchmarkStatusRunning}},
			UpdateBenchmarkResult: &UpdateBenchmarkResult{err: assert.AnError},
			isErr:                 true,
			err:                   assert.AnError,
		},
		"UpdateBenchmarkLogでエラーなのでエラー": {
			benchmarkID:              uuid.New(),
			benchLog:                 benchLog,
			score:                    0,
			startedAt:                time.Now(),
			FindBenchmarkResult:      &FindBenchmarkResult{benchmark: domain.Benchmark{Status: domain.BenchmarkStatusRunning}},
			UpdateBenchmarkResult:    &UpdateBenchmarkResult{},
			UpdateBenchmarkLogResult: &UpdateBenchmarkLogResult{err: assert.AnError},
			isErr:                    true,
			err:                      assert.AnError,
		},
		"正しく更新できる": {
			benchmarkID:              uuid.New(),
			benchLog:                 benchLog,
			score:                    0,
			startedAt:                time.Now(),
			FindBenchmarkResult:      &FindBenchmarkResult{benchmark: domain.Benchmark{Status: domain.BenchmarkStatusRunning}},
			UpdateBenchmarkResult:    &UpdateBenchmarkResult{},
			UpdateBenchmarkLogResult: &UpdateBenchmarkLogResult{},
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			repo := mock.NewMockRepository(ctrl)
			repo.EXPECT().Transaction(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(context.Context) error) error {
					return f(ctx)
				})
			if testCase.FindBenchmarkResult != nil {
				repo.EXPECT().FindBenchmark(gomock.Any(), testCase.benchmarkID).
					Return(testCase.FindBenchmarkResult.benchmark, testCase.FindBenchmarkResult.err)
			}
			if testCase.UpdateBenchmarkResult != nil {
				repo.EXPECT().UpdateBenchmark(gomock.Any(), testCase.benchmarkID, domain.Benchmark{
					ID:        testCase.benchmarkID,
					Instance:  testCase.FindBenchmarkResult.benchmark.Instance,
					TeamID:    testCase.FindBenchmarkResult.benchmark.TeamID,
					UserID:    testCase.FindBenchmarkResult.benchmark.UserID,
					Status:    domain.BenchmarkStatusRunning,
					CreatedAt: testCase.FindBenchmarkResult.benchmark.CreatedAt,
					StartedAt: &testCase.startedAt,
					Score:     testCase.score,
				}).
					Return(testCase.UpdateBenchmarkResult.err)
			}
			if testCase.UpdateBenchmarkLogResult != nil {
				repo.EXPECT().UpdateBenchmarkLog(gomock.Any(), testCase.benchmarkID, testCase.benchLog).
					Return(testCase.UpdateBenchmarkLogResult.err)
			}

			uc := usecase.NewBenchmarkUseCase(repo, nil, nil, "")

			err := uc.SaveBenchmarkProgress(t.Context(), testCase.benchmarkID, testCase.benchLog, testCase.score, testCase.startedAt)
			if testCase.isErr {
				if testCase.err != nil {
					assert.ErrorIs(t, err, testCase.err)
				} else if testCase.isUseCaseError {
					assert.True(t, usecase.IsUseCaseError(err))
				} else {
					assert.Error(t, err)
				}

			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFinalizeBenchmark(t *testing.T) {
	t.Parallel()

	type (
		FindBenchmarkResult struct {
			benchmark domain.Benchmark
			err       error
		}
		UpdateBenchmarkResult struct {
			err error
		}
	)

	benchmark := domain.Benchmark{
		ID:        uuid.New(),
		Instance:  domain.Instance{ID: uuid.New()},
		TeamID:    uuid.New(),
		UserID:    uuid.New(),
		Status:    domain.BenchmarkStatusRunning,
		CreatedAt: time.Now(),
		StartedAt: ptr.Of(time.Now()),
		Score:     100,
	}

	testCases := map[string]struct {
		benchmarkID           uuid.UUID
		result                domain.BenchmarkResult
		finishedAt            time.Time
		errorMes              *string
		FindBenchmarkResult   *FindBenchmarkResult
		UpdateBenchmarkResult *UpdateBenchmarkResult
		isErr                 bool
		err                   error
		isUseCaseError        bool
	}{
		"FindBenchmarkでErrNotFoundなのでErrNotFound": {
			benchmarkID:         uuid.New(),
			result:              domain.BenchmarkResultStatusPassed,
			finishedAt:          time.Now(),
			FindBenchmarkResult: &FindBenchmarkResult{err: repository.ErrNotFound},
			isErr:               true,
			err:                 usecase.ErrNotFound,
		},
		"FindBenchmarkでErrNotFound以外のエラーが返ってきたのでエラー": {
			benchmarkID:         uuid.New(),
			result:              domain.BenchmarkResultStatusPassed,
			finishedAt:          time.Now(),
			FindBenchmarkResult: &FindBenchmarkResult{err: assert.AnError},
			isErr:               true,
			err:                 assert.AnError,
		},
		"benchmarkがrunningでないのでUseCaseError": {
			benchmarkID:         uuid.New(),
			result:              domain.BenchmarkResultStatusPassed,
			finishedAt:          time.Now(),
			FindBenchmarkResult: &FindBenchmarkResult{benchmark: domain.Benchmark{Status: domain.BenchmarkStatusWaiting}},
			isErr:               true,
			isUseCaseError:      true,
		},
		"UpdateBenchmarkでエラーなのでエラー": {
			benchmarkID:           uuid.New(),
			result:                domain.BenchmarkResultStatusPassed,
			finishedAt:            time.Now(),
			FindBenchmarkResult:   &FindBenchmarkResult{benchmark: benchmark},
			UpdateBenchmarkResult: &UpdateBenchmarkResult{err: assert.AnError},
			isErr:                 true,
			err:                   assert.AnError,
		},
		"正しく更新できる": {
			benchmarkID:           uuid.New(),
			result:                domain.BenchmarkResultStatusPassed,
			finishedAt:            time.Now(),
			FindBenchmarkResult:   &FindBenchmarkResult{benchmark: benchmark},
			UpdateBenchmarkResult: &UpdateBenchmarkResult{},
			isErr:                 false,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			repo := mock.NewMockRepository(ctrl)
			repo.EXPECT().Transaction(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(context.Context) error) error {
					return f(ctx)
				},
				)

			if testCase.FindBenchmarkResult != nil {
				repo.EXPECT().FindBenchmark(gomock.Any(), testCase.benchmarkID).
					Return(testCase.FindBenchmarkResult.benchmark, testCase.FindBenchmarkResult.err)
			}
			if testCase.UpdateBenchmarkResult != nil {
				repo.EXPECT().UpdateBenchmark(gomock.Any(), testCase.benchmarkID, domain.Benchmark{
					ID:         testCase.benchmarkID,
					Instance:   testCase.FindBenchmarkResult.benchmark.Instance,
					TeamID:     testCase.FindBenchmarkResult.benchmark.TeamID,
					UserID:     testCase.FindBenchmarkResult.benchmark.UserID,
					Status:     domain.BenchmarkStatusFinished,
					CreatedAt:  testCase.FindBenchmarkResult.benchmark.CreatedAt,
					StartedAt:  testCase.FindBenchmarkResult.benchmark.StartedAt,
					FinishedAt: &testCase.finishedAt,
					Score:      testCase.FindBenchmarkResult.benchmark.Score,
					Result:     &testCase.result,
					ErrorMes:   testCase.errorMes,
				}).
					Return(testCase.UpdateBenchmarkResult.err)
			}

			b := usecase.NewBenchmarkUseCase(repo, nil, nil, "")

			err := b.FinalizeBenchmark(t.Context(), testCase.benchmarkID, testCase.result, testCase.finishedAt, testCase.errorMes)

			if testCase.isErr {
				if testCase.err != nil {
					assert.ErrorIs(t, err, testCase.err)
				} else if testCase.isUseCaseError {
					assert.True(t, usecase.IsUseCaseError(err))
				} else {
					assert.Error(t, err)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestStartBenchmark(t *testing.T) {
	t.Parallel()

	ctrl := gomock.NewController(t)

	targetIP := "10.0.0.1"
	bench := domain.Benchmark{
		ID: uuid.New(),
		Instance: domain.Instance{ID: uuid.New(), Infra: domain.InfraInstance{
			ProviderInstanceID: "i-target", PrivateIP: &targetIP,
		}},
		TeamID:    uuid.New(),
		UserID:    uuid.New(),
		Status:    domain.BenchmarkStatusWaiting,
		CreatedAt: time.Now(),
	}
	afterBench := domain.Benchmark{
		ID:        bench.ID,
		Instance:  bench.Instance,
		TeamID:    bench.TeamID,
		UserID:    bench.UserID,
		Status:    domain.BenchmarkStatusReadying,
		CreatedAt: bench.CreatedAt,
	}

	testCases := map[string]struct {
		oldestBench                 domain.Benchmark
		GetOldestQueuedBenchmarkErr error
		executeGetInfraInstance     bool
		GetInfraInstanceErr         error
		executeUpdateBenchmark      bool
		UpdateBenchmarkErr          error
		bench                       domain.Benchmark
		err                         error
	}{
		"GetOldestQueuedBenchmarkがエラーなのでエラー": {
			GetOldestQueuedBenchmarkErr: assert.AnError,
			err:                         assert.AnError,
		},
		"GetOldestQueuedBenchmarkがErrNotFoundなのでErrNotFound": {
			GetOldestQueuedBenchmarkErr: repository.ErrNotFound,
			err:                         usecase.ErrNotFound,
		},
		"GetInfraInstanceがエラーなのでエラー": {
			oldestBench:             bench,
			executeGetInfraInstance: true,
			GetInfraInstanceErr:     assert.AnError,
			err:                     assert.AnError,
		},
		"UpdateBenchmarkがエラーなのでエラー": {
			oldestBench:             bench,
			executeGetInfraInstance: true,
			executeUpdateBenchmark:  true,
			UpdateBenchmarkErr:      assert.AnError,
			err:                     assert.AnError,
		},
		"UpdateBenchmarkがErrNotFoundなのでErrNotFound": {
			oldestBench:             bench,
			executeGetInfraInstance: true,
			executeUpdateBenchmark:  true,
			UpdateBenchmarkErr:      repository.ErrNotFound,
			err:                     usecase.ErrNotFound,
		},
		"正しく実行できる": {
			oldestBench:             bench,
			executeGetInfraInstance: true,
			executeUpdateBenchmark:  true,
			bench:                   afterBench,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			repoMock := mock.NewMockRepository(ctrl)
			infraMock := instanceMock.NewMockManager(ctrl)

			repoMock.EXPECT().Transaction(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(context.Context) error) error {
					return f(ctx)
				})
			repoMock.EXPECT().
				GetOldestQueuedBenchmark(gomock.Any()).
				Return(testCase.oldestBench, testCase.GetOldestQueuedBenchmarkErr)
			if testCase.executeGetInfraInstance {
				infraMock.EXPECT().
					GetByIDs(gomock.Any(), []string{testCase.oldestBench.Instance.Infra.ProviderInstanceID}).
					Return([]domain.InfraInstance{testCase.oldestBench.Instance.Infra}, testCase.GetInfraInstanceErr)
				if testCase.GetInfraInstanceErr == nil {
					repoMock.EXPECT().GetTeamInstances(gomock.Any(), bench.TeamID, false).
						Return([]domain.Instance{bench.Instance}, nil)
				}
			}
			if testCase.executeUpdateBenchmark {
				bench := testCase.oldestBench
				bench.Status = domain.BenchmarkStatusReadying
				repoMock.EXPECT().
					UpdateBenchmark(gomock.Any(), bench.ID, bench).
					Return(testCase.UpdateBenchmarkErr)
			}

			b := usecase.NewBenchmarkUseCase(repoMock, nil, infraMock, "")

			bench, err := b.StartBenchmark(t.Context())

			if testCase.err != nil {
				assert.ErrorIs(t, err, testCase.err)
			} else {
				assert.NoError(t, err)
			}

			if err != nil {
				return
			}

			testutil.CompareBenchmark(t, testCase.bench, bench.Benchmark)
			assert.Equal(t, []string{targetIP}, bench.InstanceIPAddresses)
		})
	}
}

func TestStartBenchmarkInstanceIPAddresses(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name     string
		teamErr  error
		infraErr error
	}{
		{name: "同じチームの稼働中サーバーのprivate IPを取得する"},
		{name: "チームの一覧取得に失敗したら開始しない", teamErr: assert.AnError},
		{name: "インフラの一覧取得に失敗したら開始しない", infraErr: assert.AnError},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repo := mock.NewMockRepository(ctrl)
			manager := instanceMock.NewMockManager(ctrl)
			target := domain.InfraInstance{
				ProviderInstanceID: "i-target", PrivateIP: ptr.Of("10.0.0.1"),
				PublicIP: ptr.Of("192.0.2.1"), Status: domain.InstanceStatusRunning,
			}
			bench := domain.Benchmark{
				ID: uuid.New(), TeamID: uuid.New(),
				Instance: domain.Instance{ID: uuid.New(), Infra: target},
				Status:   domain.BenchmarkStatusWaiting,
			}
			instances := []domain.Instance{bench.Instance}
			ids := []string{"i-other-2", "i-other-3", "i-no-ip", "i-empty-ip", "i-deleted", "i-deleting", "i-duplicate"}
			for _, id := range ids {
				instances = append(instances, domain.Instance{
					TeamID: bench.TeamID, Infra: domain.InfraInstance{ProviderInstanceID: id},
				})
			}
			infraInstances := []domain.InfraInstance{
				{ProviderInstanceID: "i-other-2", PrivateIP: ptr.Of("10.0.0.2"), PublicIP: ptr.Of("192.0.2.2"), Status: domain.InstanceStatusRunning},
				{ProviderInstanceID: "i-other-3", PrivateIP: ptr.Of("10.0.0.3"), Status: domain.InstanceStatusStopped},
				{ProviderInstanceID: "i-no-ip", Status: domain.InstanceStatusRunning},
				{ProviderInstanceID: "i-empty-ip", PrivateIP: ptr.Of(""), Status: domain.InstanceStatusRunning},
				{ProviderInstanceID: "i-deleted", PrivateIP: ptr.Of("10.0.0.4"), Status: domain.InstanceStatusDeleted},
				{ProviderInstanceID: "i-deleting", PrivateIP: ptr.Of("10.0.0.5"), Status: domain.InstanceStatusDeleting},
				{ProviderInstanceID: "i-duplicate", PrivateIP: ptr.Of("10.0.0.2"), Status: domain.InstanceStatusRunning},
			}
			repo.EXPECT().Transaction(gomock.Any(), gomock.Any()).DoAndReturn(
				func(ctx context.Context, f func(context.Context) error) error { return f(ctx) },
			)
			calls := []any{
				repo.EXPECT().GetOldestQueuedBenchmark(gomock.Any()).Return(bench, nil),
				manager.EXPECT().GetByIDs(gomock.Any(), []string{"i-target"}).Return([]domain.InfraInstance{target}, nil),
				repo.EXPECT().GetTeamInstances(gomock.Any(), bench.TeamID, false).Return(instances, tt.teamErr),
			}
			if tt.teamErr == nil {
				calls = append(calls, manager.EXPECT().GetByIDs(gomock.Any(), ids).Return(infraInstances, tt.infraErr))
			}
			wantErr := tt.teamErr != nil || tt.infraErr != nil
			if !wantErr {
				readying := bench
				readying.Status = domain.BenchmarkStatusReadying
				calls = append(calls, repo.EXPECT().UpdateBenchmark(gomock.Any(), bench.ID, readying).Return(nil))
			}
			gomock.InOrder(calls...)

			job, err := usecase.NewBenchmarkUseCase(repo, instanceMock.NewMockEnvironmentManager(ctrl), manager, "").StartBenchmark(t.Context())
			if wantErr {
				require.ErrorIs(t, err, assert.AnError)
				assert.Equal(t, domain.BenchmarkJob{}, job)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, []string{"10.0.0.1", "10.0.0.2"}, job.InstanceIPAddresses)
			assert.Equal(t, "10.0.0.1", *job.Benchmark.Instance.Infra.PrivateIP)
			assert.Equal(t, domain.BenchmarkStatusReadying, job.Benchmark.Status)
		})
	}
}

// 対象サーバーが消えたベンチを待ち行列に残すと、後ろのベンチがすべて始められなくなる。
func TestStartBenchmarkTargetGone(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct {
		name  string
		found []domain.InfraInstance
	}{
		{name: "対象サーバーが見つからない"},
		{name: "対象サーバーが削除済み", found: []domain.InfraInstance{
			{ProviderInstanceID: "i-gone", PrivateIP: ptr.Of("10.0.0.1"), Status: domain.InstanceStatusDeleted},
		}},
		{name: "対象サーバーに private IP がない", found: []domain.InfraInstance{
			{ProviderInstanceID: "i-gone", Status: domain.InstanceStatusRunning},
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctrl := gomock.NewController(t)
			repo := mock.NewMockRepository(ctrl)
			manager := instanceMock.NewMockManager(ctrl)
			gone := domain.Benchmark{
				ID: uuid.New(), TeamID: uuid.New(),
				Instance: domain.Instance{ID: uuid.New(), Infra: domain.InfraInstance{ProviderInstanceID: "i-gone"}},
				Status:   domain.BenchmarkStatusWaiting,
			}

			repo.EXPECT().Transaction(gomock.Any(), gomock.Any()).Times(2).DoAndReturn(
				func(ctx context.Context, f func(context.Context) error) error { return f(ctx) },
			)
			gomock.InOrder(
				repo.EXPECT().GetOldestQueuedBenchmark(gomock.Any()).Return(gone, nil),
				manager.EXPECT().GetByIDs(gomock.Any(), []string{"i-gone"}).Return(tt.found, nil),
				repo.EXPECT().UpdateBenchmark(gomock.Any(), gone.ID, gomock.Any()).DoAndReturn(
					func(_ context.Context, _ uuid.UUID, b domain.Benchmark) error {
						assert.Equal(t, domain.BenchmarkStatusFinished, b.Status)
						assert.Equal(t, domain.BenchmarkResultStatusError, *b.Result)
						assert.NotNil(t, b.StartedAt)
						assert.NotNil(t, b.FinishedAt)
						assert.NotNil(t, b.ErrorMes)
						return nil
					}),
				repo.EXPECT().GetOldestQueuedBenchmark(gomock.Any()).Return(domain.Benchmark{}, repository.ErrNotFound),
			)

			_, err := usecase.NewBenchmarkUseCase(repo, instanceMock.NewMockEnvironmentManager(ctrl), manager, "").StartBenchmark(t.Context())
			assert.ErrorIs(t, err, usecase.ErrNotFound)
		})
	}
}

func TestDeleteBenchmark(t *testing.T) {
	t.Parallel()

	testCases := map[string]struct {
		benchmarkID            uuid.UUID
		DeleteBenchmarkLogsErr error
		executeDeleteBenchmark bool
		DeleteBenchmarkErr     error
		err                    error
	}{
		"DeleteBenchmarkLogsがエラーなのでエラー": {
			benchmarkID:            uuid.New(),
			DeleteBenchmarkLogsErr: assert.AnError,
			err:                    assert.AnError,
		},
		"DeleteBenchmarkがエラーなのでエラー": {
			benchmarkID:            uuid.New(),
			executeDeleteBenchmark: true,
			DeleteBenchmarkErr:     assert.AnError,
			err:                    assert.AnError,
		},
		"DeleteBenchmarkがErrNotFoundなのでErrNotFound": {
			benchmarkID:            uuid.New(),
			executeDeleteBenchmark: true,
			DeleteBenchmarkErr:     repository.ErrNotFound,
			err:                    usecase.ErrNotFound,
		},
		"benchmarkを正しく削除できる": {
			benchmarkID:            uuid.New(),
			executeDeleteBenchmark: true,
		},
		"DeleteBenchmarkLogsがErrNotFoundでも削除できる": {
			benchmarkID:            uuid.New(),
			DeleteBenchmarkLogsErr: repository.ErrNotFound,
			executeDeleteBenchmark: true,
		},
	}

	for name, testCase := range testCases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)

			repoMock := mock.NewMockRepository(ctrl)

			repoMock.EXPECT().Transaction(gomock.Any(), gomock.Any()).
				DoAndReturn(func(ctx context.Context, f func(context.Context) error) error {
					return f(ctx)
				})
			repoMock.EXPECT().
				DeleteBenchmarkLogs(gomock.Any(), testCase.benchmarkID).
				Return(testCase.DeleteBenchmarkLogsErr)
			if testCase.executeDeleteBenchmark {
				repoMock.EXPECT().
					DeleteBenchmark(gomock.Any(), testCase.benchmarkID).
					Return(testCase.DeleteBenchmarkErr)
			}

			b := usecase.NewBenchmarkUseCase(repoMock, nil, nil, "")

			err := b.DeleteBenchmark(t.Context(), testCase.benchmarkID)

			if err != nil {
				assert.ErrorIs(t, err, testCase.err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
