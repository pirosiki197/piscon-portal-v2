package aws

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/traPtitech/piscon-portal-v2/server/services/instance"
)

func TestTemplateParams(t *testing.T) {
	t.Run("ポータルが渡すパラメータと既定値のあるパラメータだけなら通す", func(t *testing.T) {
		params, err := templateParams(&cloudformation.ValidateTemplateOutput{
			Parameters: []cfntypes.TemplateParameter{
				{ParameterKey: aws.String("ImageId")},
				{ParameterKey: aws.String("InstanceType"), DefaultValue: aws.String("c7a.large")},
			},
		})
		require.NoError(t, err)
		assert.Equal(t, map[string]bool{"ImageId": true, "InstanceType": false}, params)
	})

	t.Run("既定値のない知らないパラメータがあればエラー", func(t *testing.T) {
		_, err := templateParams(&cloudformation.ValidateTemplateOutput{
			Parameters: []cfntypes.TemplateParameter{{ParameterKey: aws.String("KeyName")}},
		})
		assert.ErrorContains(t, err, "KeyName")
	})

	t.Run("capability が要るテンプレートはエラー", func(t *testing.T) {
		_, err := templateParams(&cloudformation.ValidateTemplateOutput{
			Capabilities: []cfntypes.Capability{cfntypes.CapabilityCapabilityIam},
		})
		assert.Error(t, err)
	})
}

func TestStackParameters(t *testing.T) {
	t.Run("テンプレートが宣言しているパラメータだけを渡す", func(t *testing.T) {
		params, err := stackParameters(
			map[string]bool{"ImageId": true, "InstanceType": false},
			map[string]string{"ImageId": "ami-1", "InstanceType": "c7a.large", "TeamId": "team"},
		)
		require.NoError(t, err)
		assert.Equal(t, []cfntypes.Parameter{{ParameterKey: aws.String("ImageId"), ParameterValue: aws.String("ami-1")}}, params)
	})

	t.Run("上限より長い値はエラー", func(t *testing.T) {
		_, err := stackParameters(
			map[string]bool{"SshPublicKeys": true},
			map[string]string{"SshPublicKeys": strings.Repeat("a", maxParameterValueBytes+1)},
		)
		assert.ErrorIs(t, err, instance.ErrInvalidEnvironmentParams)
	})

	t.Run("宣言していないパラメータは長くても渡さないのでエラーにしない", func(t *testing.T) {
		params, err := stackParameters(
			map[string]bool{},
			map[string]string{"SshPublicKeys": strings.Repeat("a", maxParameterValueBytes+1)},
		)
		require.NoError(t, err)
		assert.Empty(t, params)
	})
}
