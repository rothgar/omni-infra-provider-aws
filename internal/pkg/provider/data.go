package provider

import (
	"hash/fnv"
)

// Data is the provider custom machine config (Machine Class parameters in Omni).
type Data struct {
	InstanceType     string   `yaml:"instance_type"`
	SubnetID         string   `yaml:"subnet_id,omitempty"`          // Single subnet (backward compatible)
	SubnetIDs        []string `yaml:"subnet_ids,omitempty"`         // Multiple subnets (for HA across AZs)
	SecurityGroupIDs []string `yaml:"security_group_ids,omitempty"`
	VolumeSize       int64    `yaml:"volume_size,omitempty"`
	Arch             string   `yaml:"arch,omitempty"` // Default to amd64
}

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
