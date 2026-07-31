package provider

import (
	"hash/fnv"
)

// Data is the provider custom machine config (Machine Class parameters in Omni).
type Data struct {
	InstanceType           string   `yaml:"instance_type"`
	SubnetID               string   `yaml:"subnet_id,omitempty"`  // Single subnet (backward compatible)
	SubnetIDs              []string `yaml:"subnet_ids,omitempty"` // Multiple subnets (for HA across AZs)
	SecurityGroupIDs       []string `yaml:"security_group_ids,omitempty"`
	VolumeSize             int64    `yaml:"volume_size,omitempty"`
	Arch                   string   `yaml:"arch,omitempty"`                       // Default to amd64
	IamInstanceProfileArn  string   `yaml:"iam_instance_profile_arn,omitempty"`   // ARN form; takes precedence over Name
	IamInstanceProfileName string   `yaml:"iam_instance_profile_name,omitempty"`  // Name form; defaults to AmazonEKSNodeRole
}

// DefaultIamInstanceProfileName is AWS's documented naming convention for EKS
// worker node IAM roles. The role isn't provisioned by AWS automatically —
// users must create it in IAM with these attached policies before it will
// work: AmazonEKSWorkerNodePolicy, AmazonEKS_CNI_Policy,
// AmazonEC2ContainerRegistryReadOnly, AmazonEBSCSIDriverPolicy.
// See https://docs.aws.amazon.com/eks/latest/userguide/create-node-role.html
const DefaultIamInstanceProfileName = "AmazonEKSNodeRole"

// GetSubnetID returns a subnet ID, using hash-based selection for even distribution across AZs
// The requestID is used to deterministically select a subnet, ensuring even distribution
func (d *Data) GetSubnetID(requestID string) string {
	// If SubnetIDs is specified, use hash-based selection for distribution
	if len(d.SubnetIDs) > 0 {
		// Use FNV hash of requestID to select subnet
		h := fnv.New32a()
		h.Write([]byte(requestID))
		idx := int(h.Sum32()) % len(d.SubnetIDs)
		return d.SubnetIDs[idx]
	}
	// Fall back to single SubnetID (backward compatible)
	return d.SubnetID
}
