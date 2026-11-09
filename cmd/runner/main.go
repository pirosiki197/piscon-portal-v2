package main

import (
	"log"
	"os"
	"time"

	portalv1 "github.com/traPtitech/piscon-portal-v2/gen/portal/v1"
	"github.com/traPtitech/piscon-portal-v2/runner"
	"github.com/traPtitech/piscon-portal-v2/runner/config"
	portalGrpc "github.com/traPtitech/piscon-portal-v2/runner/portal/grpc"
	grpc "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	// runner check <bundle-dir> は、問題バンドルを検証して終了する。デプロイの最後に実行する。
	if len(os.Args) == 3 && os.Args[1] == "check" {
		if _, err := runner.NewBenchmarker(config.Config{Bundle: os.Args[2]}); err != nil {
			log.Fatalf("invalid bundle: %v", err)
		}
		log.Printf("bundle is valid: %s", os.Args[2])
		return
	}

	config, err := config.LoadFile()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
		return
	}
	if err := config.Validate(); err != nil {
		log.Fatalf("invalid config: %v", err)
	}

	conn, err := grpc.NewClient(config.Portal.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials())) //TODO: TLS を有効にする
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
		return
	}
	defer conn.Close()

	client := portalv1.NewBenchmarkServiceClient(conn)
	p := portalGrpc.NewPortal(client, time.Second)

	r, err := runner.Prepare(p, config)
	if err != nil {
		log.Fatalf("failed to prepare runner: %v", err)
		return
	}

	log.Printf("runner started: portal=%s, bundle=%s, problem=%s\n", config.Portal.Address, config.Bundle, config.Problem.Name)

	for {
		if err := r.Run(); err != nil {
			log.Printf("error: %v", err)
		}
	}
}
