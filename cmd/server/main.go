package main

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/traPtitech/piscon-portal-v2/bundle"
	"github.com/traPtitech/piscon-portal-v2/server/handler"
	"github.com/traPtitech/piscon-portal-v2/server/repository"
	dbrepo "github.com/traPtitech/piscon-portal-v2/server/repository/db"
	"github.com/traPtitech/piscon-portal-v2/server/server"
	"github.com/traPtitech/piscon-portal-v2/server/services/auth/dev"
	"github.com/traPtitech/piscon-portal-v2/server/services/auth/traq"
	"github.com/traPtitech/piscon-portal-v2/server/services/github"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance/aws"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance/fake"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance/incus"
	"github.com/traPtitech/piscon-portal-v2/server/usecase"
)

func main() {
	e := echo.New()

	e.Use(middleware.RequestLogger())

	authProviders, err := provideAuthProviders()
	if err != nil {
		log.Fatal("failed to create auth providers:", err)
	}
	handlerConfig := handler.Config{
		SessionSecret: os.Getenv("SESSION_SECRET"),
		AuthProviders: authProviders,
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
	handler := handler.New(useCase, repo, handlerConfig)
	handler.SetupRoutes(e)

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
	if manager == "incus" {
		return provideIncusManagers(problemBundle)
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

func provideIncusManagers(problemBundle *bundle.Bundle) (instance.Manager, instance.EnvironmentManager, error) {
	if problemBundle == nil {
		return nil, nil, errors.New("PROBLEM_BUNDLE_DIR is required for INSTANCE_MANAGER=incus")
	}
	env := problemBundle.Environment
	if env.Incus == nil {
		return nil, nil, fmt.Errorf("bundle %q has no environment.incus", problemBundle.Name)
	}
	userData, err := os.ReadFile(env.Incus.UserDataPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read user-data: %w", err)
	}
	socket := os.Getenv("INCUS_SOCKET")
	if socket == "" {
		return nil, nil, errors.New("INCUS_SOCKET is required for INSTANCE_MANAGER=incus")
	}

	client, err := incus.New(context.Background(), incus.Config{
		SocketPath: socket,
		Servers:    env.Servers,
		Image:      env.Incus.Image,
		UserData:   string(userData),
		CPU:        env.Incus.CPU,
		Memory:     env.Incus.Memory,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create incus manager: %w", err)
	}
	return client, client, nil
}

// provideAuthProviders は、AUTH_PROVIDERS にカンマ区切りで並べた認証手段を作る。AUTH_PROVIDERS が空なら traQ だけを作る。
func provideAuthProviders() ([]handler.AuthProvider, error) {
	ids := strings.Split(cmp.Or(os.Getenv("AUTH_PROVIDERS"), traq.ProviderID), ",")
	providers := make([]handler.AuthProvider, 0, len(ids))
	for _, id := range ids {
		switch strings.TrimSpace(id) {
		case traq.ProviderID:
			callbackURL, err := url.JoinPath(os.Getenv("ROOT_URL"), handler.CallbackPath(traq.ProviderID))
			if err != nil {
				return nil, fmt.Errorf("join traQ callback URL: %w", err)
			}
			p, err := traq.New(context.Background(), traq.Config{
				Issuer:       "https://q.trap.jp",
				ClientID:     os.Getenv("CLIENT_ID"),
				ClientSecret: os.Getenv("CLIENT_SECRET"),
				AuthURL:      "https://q.trap.jp/api/v3/oauth2/authorize",
				TokenURL:     "https://q.trap.jp/api/v3/oauth2/token",
			}, callbackURL)
			if err != nil {
				return nil, fmt.Errorf("create traQ auth provider: %w", err)
			}
			providers = append(providers, p)
		case dev.ProviderID:
			log.Println("WARNING: dev login is enabled. Anyone can log in as any user.")
			providers = append(providers, dev.New(handler.AuthorizePagePath(dev.ProviderID), handler.CallbackPath(dev.ProviderID)))
		default:
			return nil, fmt.Errorf("unknown auth provider in AUTH_PROVIDERS: %q", id)
		}
	}
	return providers, nil
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
