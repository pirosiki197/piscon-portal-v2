package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
