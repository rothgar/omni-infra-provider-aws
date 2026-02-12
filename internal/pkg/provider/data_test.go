package provider

import (
	"fmt"
	"testing"
)

func TestGetSubnetID_Distribution(t *testing.T) {
	tests := []struct {
		name           string
		numSubnets     int
		numRequests    int
		expectEvenDist bool
	}{
		{"1 subnet", 1, 10, true},
		{"2 subnets", 2, 20, true},
		{"3 subnets", 3, 30, true},
		{"4 subnets", 4, 40, true},
		{"5 subnets", 5, 50, true},
		{"6 subnets", 6, 60, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create subnet IDs
			subnetIDs := make([]string, tt.numSubnets)
			for i := 0; i < tt.numSubnets; i++ {
				subnetIDs[i] = fmt.Sprintf("subnet-%d", i)
			}

			data := Data{
				SubnetIDs: subnetIDs,
			}

			// Count distribution
			distribution := make(map[string]int)
			for i := 0; i < tt.numRequests; i++ {
				requestID := fmt.Sprintf("request-%d", i)
				subnet := data.GetSubnetID(requestID)
				distribution[subnet]++
			}

			// Verify all subnets are used
			if len(distribution) != tt.numSubnets {
				t.Errorf("Expected %d subnets to be used, got %d", tt.numSubnets, len(distribution))
			}

			// Calculate expected count per subnet
			expectedPerSubnet := tt.numRequests / tt.numSubnets
			tolerance := 2 // Allow +/- 2 for small variations

			// Verify distribution is roughly even
			for subnet, count := range distribution {
				diff := count - expectedPerSubnet
				if diff < 0 {
					diff = -diff
				}
				if diff > tolerance {
					t.Logf("Subnet %s: %d requests (expected ~%d)", subnet, count, expectedPerSubnet)
				}
			}

			t.Logf("Distribution for %d subnets with %d requests:", tt.numSubnets, tt.numRequests)
			for i, subnetID := range subnetIDs {
				count := distribution[subnetID]
				percentage := float64(count) / float64(tt.numRequests) * 100
				t.Logf("  Subnet %d: %d requests (%.1f%%)", i, count, percentage)
			}
		})
	}
}

func TestGetSubnetID_Deterministic(t *testing.T) {
	data := Data{
		SubnetIDs: []string{"subnet-a", "subnet-b", "subnet-c"},
	}

	// Same request ID should always return same subnet
	requestID := "test-request-123"
	first := data.GetSubnetID(requestID)

	for i := 0; i < 100; i++ {
		result := data.GetSubnetID(requestID)
		if result != first {
			t.Errorf("GetSubnetID is not deterministic: expected %s, got %s", first, result)
		}
	}
}

func TestGetSubnetID_BackwardCompatibility(t *testing.T) {
	// Test single SubnetID (legacy)
	data := Data{
		SubnetID: "subnet-legacy",
	}

	result := data.GetSubnetID("any-request-id")
	if result != "subnet-legacy" {
		t.Errorf("Expected subnet-legacy, got %s", result)
	}

	// Test empty subnets
	data2 := Data{}
	result2 := data2.GetSubnetID("any-request-id")
	if result2 != "" {
		t.Errorf("Expected empty string, got %s", result2)
	}
}

func TestGetSubnetID_SingleSubnetInArray(t *testing.T) {
	data := Data{
		SubnetIDs: []string{"subnet-only"},
	}

	// Should always return the only subnet
	for i := 0; i < 10; i++ {
		requestID := fmt.Sprintf("request-%d", i)
		result := data.GetSubnetID(requestID)
		if result != "subnet-only" {
			t.Errorf("Expected subnet-only, got %s", result)
		}
	}
}

// Benchmark distribution performance
func BenchmarkGetSubnetID(b *testing.B) {
	data := Data{
		SubnetIDs: []string{"subnet-a", "subnet-b", "subnet-c", "subnet-d", "subnet-e"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		requestID := fmt.Sprintf("request-%d", i)
		data.GetSubnetID(requestID)
	}
}
