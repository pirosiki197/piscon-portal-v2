package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/samber/lo"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
)

var _ instance.Manager = (*Client)(nil)

type Client struct {
	client *ec2.Client
}

// Config は、AWS に接続するための設定である。
type Config struct {
	Region string

	AccessKey string
	SecretKey string
}

func loadConfig(ctx context.Context, cfg Config) (aws.Config, error) {
	opts := []func(*config.LoadOptions) error{config.WithRegion(cfg.Region)}
	if cfg.AccessKey != "" && cfg.SecretKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.StaticCredentialsProvider{
				Value: aws.Credentials{
					AccessKeyID:     cfg.AccessKey,
					SecretAccessKey: cfg.SecretKey,
				},
			},
		))
	}
	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return aws.Config{}, fmt.Errorf("load AWS config: %w", err)
	}
	return awsCfg, nil
}

func NewClient(cfg Config) (*Client, error) {
	awsCfg, err := loadConfig(context.Background(), cfg)
	if err != nil {
		return nil, err
	}
	return &Client{client: ec2.NewFromConfig(awsCfg)}, nil
}

var pisconTagFilter = types.Filter{
	Name:   lo.ToPtr("tag:piscon"),
	Values: []string{"true"},
}

func (a *Client) Get(ctx context.Context, id string) (domain.InfraInstance, error) {
	res, err := a.client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: []string{id},
	})
	if err != nil {
		return domain.InfraInstance{}, fmt.Errorf("describe instance: %w", err)
	}

	if len(res.Reservations) == 0 || len(res.Reservations[0].Instances) == 0 {
		return domain.InfraInstance{}, fmt.Errorf("instance not found: %s", id)
	}

	instance := res.Reservations[0].Instances[0]

	return domain.InfraInstance{
		ProviderInstanceID: *instance.InstanceId,
		PrivateIP:          instance.PrivateIpAddress,
		PublicIP:           instance.PublicIpAddress,
		Status:             convertInstanceState(instance.State.Name),
	}, nil
}

func (a *Client) GetByIDs(ctx context.Context, ids []string) ([]domain.InfraInstance, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	res, err := a.client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
		InstanceIds: ids,
		Filters:     []types.Filter{pisconTagFilter},
	})
	if err != nil {
		return nil, fmt.Errorf("describe instances: %w", err)
	}

	instances := make([]domain.InfraInstance, 0, len(res.Reservations))
	for _, reservation := range res.Reservations {
		for _, instance := range reservation.Instances {
			infraInstance := domain.InfraInstance{
				ProviderInstanceID: *instance.InstanceId,
				PrivateIP:          instance.PrivateIpAddress,
				PublicIP:           instance.PublicIpAddress,
				Status:             convertInstanceState(instance.State.Name),
			}

			instances = append(instances, infraInstance)
		}
	}

	return instances, nil
}

func (a *Client) Stop(ctx context.Context, instance domain.InfraInstance) error {
	_, err := a.client.StopInstances(ctx, &ec2.StopInstancesInput{
		InstanceIds: []string{instance.ProviderInstanceID},
	})
	if err != nil {
		return fmt.Errorf("stop instance: %w", err)
	}

	return nil
}

func (a *Client) Start(ctx context.Context, instance domain.InfraInstance) error {
	_, err := a.client.StartInstances(ctx, &ec2.StartInstancesInput{
		InstanceIds: []string{instance.ProviderInstanceID},
	})
	if err != nil {
		return fmt.Errorf("start instance: %w", err)
	}

	return nil
}

func convertInstanceState(state types.InstanceStateName) domain.InstanceStatus {
	switch state {
	case types.InstanceStateNamePending:
		return domain.InstanceStatusPending
	case types.InstanceStateNameRunning:
		return domain.InstanceStatusRunning
	case types.InstanceStateNameStopping:
		return domain.InstanceStatusStopping
	case types.InstanceStateNameStopped:
		return domain.InstanceStatusStopped
	case types.InstanceStateNameShuttingDown:
		return domain.InstanceStatusDeleting
	case types.InstanceStateNameTerminated:
		return domain.InstanceStatusDeleted
	default:
		return domain.InstanceStatusUnknown
	}
}
