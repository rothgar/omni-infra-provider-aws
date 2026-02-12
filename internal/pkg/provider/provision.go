package provider

import (
	"context"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/siderolabs/omni/client/pkg/infra/provision"
	"github.com/siderolabs/omni/client/pkg/omni/resources/infra"
	"go.uber.org/zap"

	"github.com/siderolabs/omni-infra-provider-aws/internal/pkg/provider/resources"
)

type Provisioner struct {
	ec2Client *ec2.Client
	region    string
}

func NewProvisioner(ec2Client *ec2.Client, region string) *Provisioner {
	return &Provisioner{
		ec2Client: ec2Client,
		region:    region,
	}
}

func (p *Provisioner) ProvisionSteps() []provision.Step[*resources.Machine] {
	return []provision.Step[*resources.Machine]{
		provision.NewStep("lookupAMI", func(ctx context.Context, logger *zap.Logger, pctx provision.Context[*resources.Machine]) error {
			var data Data
			if err := pctx.UnmarshalProviderData(&data); err != nil {
				return err
			}

			arch := data.Arch
			if arch == "" {
				arch = "amd64"
			}

			version := pctx.GetTalosVersion()
			if version == "" {
				return fmt.Errorf("talos version is not set in machine request")
			}

			amiID, err := LookupAMI(ctx, p.region, arch, version)
			if err != nil {
				return err
			}

			pctx.State.TypedSpec().Value.AmiId = amiID
			pctx.State.TypedSpec().Value.Region = p.region
			logger.Info("looked up AMI", zap.String("ami", amiID), zap.String("version", version))
			return nil
		}),
		provision.NewStep("runInstance", func(ctx context.Context, logger *zap.Logger, pctx provision.Context[*resources.Machine]) error {
			var data Data
			if err := pctx.UnmarshalProviderData(&data); err != nil {
				return err
			}

			spec := pctx.State.TypedSpec().Value

			// Get request ID for subnet selection
			requestID := pctx.GetRequestID()

			// Get subnet ID (supports both single and multiple subnets)
			// Uses hash of requestID for deterministic distribution across AZs
			subnetID := data.GetSubnetID(requestID)

			// Validate that security groups are provided when using a subnet
			if subnetID != "" && len(data.SecurityGroupIDs) == 0 {
				return fmt.Errorf("security_group_ids must be specified when subnet_id or subnet_ids is provided")
			}

			input := &ec2.RunInstancesInput{
				ImageId:      aws.String(spec.AmiId),
				InstanceType: types.InstanceType(data.InstanceType),
				MinCount:     aws.Int32(1),
				MaxCount:     aws.Int32(1),
				UserData:     aws.String(base64.StdEncoding.EncodeToString([]byte(pctx.ConnectionParams.JoinConfig))),
				TagSpecifications: []types.TagSpecification{
					{
						ResourceType: types.ResourceTypeInstance,
						Tags: []types.Tag{
							{
								Key:   aws.String("omni-request-id"),
								Value: aws.String(pctx.GetRequestID()),
							},
							{
								Key:   aws.String("Name"),
								Value: aws.String(fmt.Sprintf("omni-%s", pctx.GetRequestID())),
							},
						},
					},
				},
			}

			if subnetID != "" {
				input.SubnetId = aws.String(subnetID)
				logger.Info("selected subnet", zap.String("subnet-id", subnetID))
			}
			if len(data.SecurityGroupIDs) > 0 {
				input.SecurityGroupIds = data.SecurityGroupIDs
			}
			if data.VolumeSize > 0 {
				input.BlockDeviceMappings = []types.BlockDeviceMapping{
					{
						DeviceName: aws.String("/dev/xvda"),
						Ebs: &types.EbsBlockDevice{
							VolumeSize: aws.Int32(int32(data.VolumeSize)),
						},
					},
				}
			}

			out, err := p.ec2Client.RunInstances(ctx, input)
			if err != nil {
				return err
			}

			if len(out.Instances) == 0 {
				return fmt.Errorf("no instances created")
			}

			instanceID := *out.Instances[0].InstanceId
			pctx.State.TypedSpec().Value.InstanceId = instanceID
			pctx.SetMachineInfraID(instanceID)

			logger.Info("instance created", zap.String("instance-id", instanceID))
			return nil
		}),
		provision.NewStep("waitRunning", func(ctx context.Context, logger *zap.Logger, pctx provision.Context[*resources.Machine]) error {
			spec := pctx.State.TypedSpec().Value
			out, err := p.ec2Client.DescribeInstances(ctx, &ec2.DescribeInstancesInput{
				InstanceIds: []string{spec.InstanceId},
			})
			if err != nil {
				return err
			}

			if len(out.Reservations) == 0 || len(out.Reservations[0].Instances) == 0 {
				return fmt.Errorf("instance %s not found", spec.InstanceId)
			}

			instance := out.Reservations[0].Instances[0]
			if instance.State.Name == types.InstanceStateNameRunning {
				return nil
			}

			if instance.State.Name == types.InstanceStateNameTerminated || instance.State.Name == types.InstanceStateNameShuttingDown {
				return fmt.Errorf("instance %s is in state %s", spec.InstanceId, instance.State.Name)
			}

			return provision.NewRetryInterval(time.Second * 10)
		}),
	}
}

func (p *Provisioner) Deprovision(ctx context.Context, logger *zap.Logger, machine *resources.Machine, machineRequest *infra.MachineRequest) error {
	instanceID := machine.TypedSpec().Value.InstanceId
	if instanceID == "" {
		logger.Warn("instance id is empty, skipping termination")
		return nil
	}

	_, err := p.ec2Client.TerminateInstances(ctx, &ec2.TerminateInstancesInput{
		InstanceIds: []string{instanceID},
	})
	if err != nil {
		return err
	}

	logger.Info("instance termination triggered", zap.String("instance-id", instanceID))
	return nil
}
