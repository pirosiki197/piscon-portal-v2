package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/go-sql-driver/mysql"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/traPtitech/piscon-portal-v2/bundle"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/handler"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	dbrepo "github.com/traPtitech/piscon-portal-v2/server/repository/db"
	"github.com/traPtitech/piscon-portal-v2/server/server"
	"github.com/traPtitech/piscon-portal-v2/server/services/github"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance/aws"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance/fake"
	"github.com/traPtitech/piscon-portal-v2/server/services/oauth2"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
)

func main() {
	e := echo.New()

	e.Use(middleware.RequestLogger())

	handlerConfig := handler.Config{
		RootURL:       os.Getenv("ROOT_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		Oauth2: oauth2.Config{
			Issuer:       "https://q.trap.jp",
			ClientID:     os.Getenv("CLIENT_ID"),
			ClientSecret: os.Getenv("CLIENT_SECRET"),
			AuthURL:      "https://q.trap.jp/api/v3/oauth2/authorize",
			TokenURL:     "https://q.trap.jp/api/v3/oauth2/token",
		},
	}

	problemBundle, err := provideBundle()
	if err != nil {
		log.Fatal("failed to load problem bundle:", err)
	}
	manager, envManager, err := provideManagers(problemBundle)
	if err != nil {
		log.Fatal("failed to create instance manager:", err)
	}
	repo, err := provideRepository()
	if err != nil {
		log.Fatal("failed to create repository:", err)
	}
	githubService := github.NewHTTPGitHubService()

	useCase := usecase.New(provideUseCaseConfig(problemBundle), repo, manager, envManager, githubService)
	handler, err := handler.New(useCase, repo, handlerConfig)
	if err != nil {
		panic(err)
	}
	handler.SetupRoutes(e)

	if err := setupAdminUser(repo); err != nil {
		log.Fatal("failed to setup admin user:", err)
	}
	if err := registerProblem(repo, problemBundle); err != nil {
		log.Fatal("failed to register problem:", err)
	}

	benchService := server.NewBenchmarkService(useCase)
	go func() {
		log.Fatal("error in bench service.", server.Start(50051, benchService))
	}()

	e.Logger.Fatal(e.Start(":8080"))
}

func provideRepository() (repository.Repository, error) {
	dbConfig := mysql.Config{
		User:      os.Getenv("DB_USER"),
		Passwd:    os.Getenv("DB_PASSWORD"),
		Net:       "tcp",
		Addr:      os.Getenv("DB_HOST") + ":" + os.Getenv("DB_PORT"),
		DBName:    os.Getenv("DB_NAME"),
		ParseTime: true,
	}
	db, err := sql.Open("mysql", dbConfig.FormatDSN())
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	return dbrepo.NewRepository(db), nil
}

func provideUseCaseConfig(problemBundle *bundle.Bundle) usecase.Config {
	if problemBundle == nil {
		return usecase.Config{}
	}
	return usecase.Config{BenchTarget: problemBundle.Bench.Target}
}

// provideBundle は、PROBLEM_BUNDLE_DIR の問題バンドルを読み込む。
// PROBLEM_BUNDLE_DIR が空なら、問題バンドルを使わずに nil を返す。
func provideBundle() (*bundle.Bundle, error) {
	dir := os.Getenv("PROBLEM_BUNDLE_DIR")
	if dir == "" {
		return nil, nil
	}
	b, err := bundle.Load(dir)
	if err != nil {
		return nil, fmt.Errorf("load bundle: %w", err)
	}
	return &b, nil
}

func provideManagers(problemBundle *bundle.Bundle) (instance.Manager, instance.EnvironmentManager, error) {
	manager := os.Getenv("INSTANCE_MANAGER")
	if manager == "aws" {
		return provideAWSManagers(problemBundle)
	}
	if manager == "" || manager == "mock" {
		return provideFakeManagers(problemBundle)
	}
	return nil, nil, fmt.Errorf("unknown INSTANCE_MANAGER: %s", manager)
}

// defaultFakeServers は、問題バンドルを使わないときに fake のチーム環境が持つサーバーの名前である。
var defaultFakeServers = []string{"Server1", "Server2", "Server3"}

func provideFakeManagers(problemBundle *bundle.Bundle) (instance.Manager, instance.EnvironmentManager, error) {
	root, err := os.OpenRoot("/app/.dev/instance")
	if err != nil {
		return nil, nil, fmt.Errorf("open root directory: %w", err)
	}
	manager, err := fake.NewManager(root)
	if err != nil {
		return nil, nil, fmt.Errorf("create fake instance manager: %w", err)
	}
	servers := defaultFakeServers
	if problemBundle != nil {
		servers = problemBundle.Environment.Servers
	}
	envManager, err := fake.NewEnvironmentManager(root, manager, servers)
	if err != nil {
		return nil, nil, fmt.Errorf("create fake environment manager: %w", err)
	}
	return manager, envManager, nil
}

func provideAWSManagers(problemBundle *bundle.Bundle) (instance.Manager, instance.EnvironmentManager, error) {
	if problemBundle == nil {
		return nil, nil, errors.New("PROBLEM_BUNDLE_DIR is required for INSTANCE_MANAGER=aws")
	}
	if problemBundle.Environment.ImageID == "" {
		return nil, nil, fmt.Errorf("bundle %q has no environment.imageId", problemBundle.Name)
	}
	template, err := os.ReadFile(problemBundle.Environment.TemplatePath)
	if err != nil {
		return nil, nil, fmt.Errorf("read environment template: %w", err)
	}

	awsConfig := aws.Config{
		Region:    os.Getenv("AWS_REGION"),
		AccessKey: os.Getenv("AWS_ACCESS_KEY"),
		SecretKey: os.Getenv("AWS_SECRET_KEY"),
	}
	manager, err := aws.NewClient(awsConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("create AWS instance manager: %w", err)
	}
	envManager, err := aws.NewEnvironmentClient(context.Background(), awsConfig, aws.EnvironmentConfig{
		TemplateBody:    string(template),
		ImageID:         problemBundle.Environment.ImageID,
		SubnetID:        os.Getenv("AWS_SUBNET_ID"),
		SecurityGroupID: os.Getenv("AWS_SECURITY_GROUP_ID"),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create AWS environment manager: %w", err)
	}
	return manager, envManager, nil
}

func setupAdminUser(repo repository.Repository) error {
	adminUserIDStr := os.Getenv("ADMIN_USER_ID")
	if adminUserIDStr == "" {
		return errors.New("ADMIN_USER_ID is not set")
	}

	adminUserID, err := uuid.Parse(adminUserIDStr)
	if err != nil {
		return fmt.Errorf("parse ADMIN_USER_ID: %w", err)
	}

	adminUserName := os.Getenv("ADMIN_USER_NAME")
	if adminUserName == "" {
		return errors.New("ADMIN_USER_NAME is not set")
	}

	ctx := context.Background()

	_, err = repo.FindUser(ctx, adminUserID)
	if err == nil {
		// user already exists
		err := repo.AddAdmins(ctx, []uuid.UUID{adminUserID})
		if err != nil {
			return fmt.Errorf("add admin role: %w", err)
		}
		return nil
	}
	if !errors.Is(err, repository.ErrNotFound) {
		return fmt.Errorf("find user: %w", err)
	}

	// user not found, create new user
	adminUser := domain.NewUser(adminUserID, adminUserName)
	adminUser.IsAdmin = true

	err = repo.CreateUser(ctx, adminUser)
	if err != nil {
		return fmt.Errorf("create admin user: %w", err)
	}
	return nil
}

// registerProblem は、問題バンドルの問題を DB に記録する。問題バンドルを使わない場合は何もしない。
func registerProblem(repo repository.Repository, problemBundle *bundle.Bundle) error {
	if problemBundle == nil {
		return nil
	}
	manual, err := os.ReadFile(problemBundle.ManualPath)
	if err != nil {
		return fmt.Errorf("read manual: %w", err)
	}
	if err := usecase.NewProblemUseCase(repo).RegisterProblem(context.Background(), problemBundle.Name, string(manual)); err != nil {
		return fmt.Errorf("bundle %q: %w", problemBundle.Name, err)
	}
	return nil
}
