package aws

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/aws/smithy-go"
	"github.com/traPtitech/piscon-portal-v2/server/domain"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
)

var _ instance.EnvironmentManager = (*EnvironmentClient)(nil)

// EnvironmentClient は、チーム環境を CloudFormation のスタックとして作る。
// プロバイダーがチーム環境を識別する値は、スタック名である。
type EnvironmentClient struct {
	client *cloudformation.Client
	cfg    EnvironmentConfig
	// params は、テンプレートが宣言しているパラメータの名前である。
	params map[string]bool
}

// EnvironmentConfig は、すべてのチーム環境に共通する、テンプレートへの入力である。
type EnvironmentConfig struct {
	TemplateBody    string
	ImageID         string
	SubnetID        string
	SecurityGroupID string
}

// テンプレートのパラメータの名前。親 doc の StackParameters に対応する。
const (
	paramImageID         = "ImageId"
	paramTeamID          = "TeamId"
	paramSSHPublicKeys   = "SshPublicKeys"
	paramSubnetID        = "SubnetId"
	paramSecurityGroupID = "SecurityGroupId"
)

var knownParams = []string{paramImageID, paramTeamID, paramSSHPublicKeys, paramSubnetID, paramSecurityGroupID}

// NewEnvironmentClient は、テンプレートを CloudFormation の検証 API で確かめてからクライアントを作る。
// テンプレートが、ポータルの渡さないパラメータを既定値なしで宣言している場合と、
// IAM などの capability を必要とする場合は、エラーを返す。
func NewEnvironmentClient(ctx context.Context, awsCfg Config, cfg EnvironmentConfig) (*EnvironmentClient, error) {
	loaded, err := loadConfig(ctx, awsCfg)
	if err != nil {
		return nil, err
	}
	client := cloudformation.NewFromConfig(loaded)

	res, err := client.ValidateTemplate(ctx, &cloudformation.ValidateTemplateInput{
		TemplateBody: aws.String(cfg.TemplateBody),
	})
	if err != nil {
		return nil, fmt.Errorf("validate template: %w", err)
	}
	params, err := templateParams(res)
	if err != nil {
		return nil, err
	}
	return &EnvironmentClient{client: client, cfg: cfg, params: params}, nil
}

func templateParams(res *cloudformation.ValidateTemplateOutput) (map[string]bool, error) {
	if len(res.Capabilities) > 0 {
		return nil, fmt.Errorf("template requires capabilities %v: %s", res.Capabilities, aws.ToString(res.CapabilitiesReason))
	}
	params := make(map[string]bool, len(res.Parameters))
	for _, p := range res.Parameters {
		key := aws.ToString(p.ParameterKey)
		known := slices.Contains(knownParams, key)
		if !known && p.DefaultValue == nil {
			return nil, fmt.Errorf("template parameter %s has no default value and the portal does not pass it; the portal passes %v", key, knownParams)
		}
		params[key] = known
	}
	return params, nil
}

func (c *EnvironmentClient) Create(ctx context.Context, name string, params instance.EnvironmentParams) (string, error) {
	values := map[string]string{
		paramImageID:         c.cfg.ImageID,
		paramTeamID:          params.TeamID,
		paramSSHPublicKeys:   strings.Join(params.SSHPublicKeys, "\n"),
		paramSubnetID:        c.cfg.SubnetID,
		paramSecurityGroupID: c.cfg.SecurityGroupID,
	}
	var parameters []cfntypes.Parameter
	for key, value := range values {
		// テンプレートが宣言していないパラメータを渡すと、スタックの作成が失敗する。
		if c.params[key] {
			parameters = append(parameters, cfntypes.Parameter{ParameterKey: aws.String(key), ParameterValue: aws.String(value)})
		}
	}

	_, err := c.client.CreateStack(ctx, &cloudformation.CreateStackInput{
		StackName:    aws.String(name),
		TemplateBody: aws.String(c.cfg.TemplateBody),
		Parameters:   parameters,
		OnFailure:    cfntypes.OnFailureRollback,
		// スタックのタグは、CloudFormation が配下のリソースにも付ける。
		Tags: []cfntypes.Tag{
			{Key: aws.String("piscon"), Value: aws.String("true")},
			{Key: aws.String("piscon:team"), Value: aws.String(params.TeamID)},
		},
	})
	if err != nil {
		return "", fmt.Errorf("create stack: %w", err)
	}
	return name, nil
}

func (c *EnvironmentClient) Get(ctx context.Context, stackName string) (domain.InfraEnvironment, error) {
	res, err := c.client.DescribeStacks(ctx, &cloudformation.DescribeStacksInput{StackName: aws.String(stackName)})
	if isStackNotFound(err) {
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted}, nil
	}
	if err != nil {
		return domain.InfraEnvironment{}, fmt.Errorf("describe stack: %w", err)
	}
	if len(res.Stacks) == 0 {
		return domain.InfraEnvironment{Status: domain.EnvironmentStatusDeleted}, nil
	}
	stack := res.Stacks[0]

	status, ok := convertStackStatus(stack.StackStatus)
	if !ok {
		return domain.InfraEnvironment{
			Status:        domain.EnvironmentStatusFailed,
			FailureReason: fmt.Sprintf("unexpected stack status %s", stack.StackStatus),
		}, nil
	}
	env := domain.InfraEnvironment{Status: status}
	switch status {
	case domain.EnvironmentStatusFailed, domain.EnvironmentStatusDeleteFailed:
		env.FailureReason = aws.ToString(stack.StackStatusReason)
	case domain.EnvironmentStatusReady:
		env.Servers, err = c.servers(ctx, stackName)
		if err != nil {
			return domain.InfraEnvironment{}, err
		}
	}
	return env, nil
}

func (c *EnvironmentClient) servers(ctx context.Context, stackName string) ([]domain.InfraServer, error) {
	var servers []domain.InfraServer
	pages := cloudformation.NewListStackResourcesPaginator(c.client, &cloudformation.ListStackResourcesInput{StackName: aws.String(stackName)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list stack resources: %w", err)
		}
		for _, r := range page.StackResourceSummaries {
			if aws.ToString(r.ResourceType) == "AWS::EC2::Instance" {
				servers = append(servers, domain.InfraServer{
					Name:               aws.ToString(r.LogicalResourceId),
					ProviderInstanceID: aws.ToString(r.PhysicalResourceId),
				})
			}
		}
	}
	return servers, nil
}

func (c *EnvironmentClient) Delete(ctx context.Context, stackName string) error {
	_, err := c.client.DeleteStack(ctx, &cloudformation.DeleteStackInput{StackName: aws.String(stackName)})
	if err != nil {
		return fmt.Errorf("delete stack: %w", err)
	}
	return nil
}

// isStackNotFound は、スタックが存在しないことを表すエラーかを判定する。
// DescribeStacks は、削除が完了したスタックをスタック名では返さず、このエラーを返す。
func isStackNotFound(err error) bool {
	apiErr, ok := errors.AsType[smithy.APIError](err)
	return ok && apiErr.ErrorCode() == "ValidationError" && strings.Contains(apiErr.ErrorMessage(), "does not exist")
}

func convertStackStatus(s cfntypes.StackStatus) (domain.EnvironmentStatus, bool) {
	switch s {
	case cfntypes.StackStatusCreateInProgress:
		return domain.EnvironmentStatusCreating, true
	case cfntypes.StackStatusCreateComplete:
		return domain.EnvironmentStatusReady, true
	case cfntypes.StackStatusCreateFailed,
		cfntypes.StackStatusRollbackInProgress,
		cfntypes.StackStatusRollbackFailed,
		cfntypes.StackStatusRollbackComplete:
		return domain.EnvironmentStatusFailed, true
	case cfntypes.StackStatusDeleteInProgress:
		return domain.EnvironmentStatusDeleting, true
	case cfntypes.StackStatusDeleteComplete:
		return domain.EnvironmentStatusDeleted, true
	case cfntypes.StackStatusDeleteFailed:
		return domain.EnvironmentStatusDeleteFailed, true
	default:
		return "", false
	}
}
