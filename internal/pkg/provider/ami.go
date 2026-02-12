package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type CloudImage struct {
	ID     string `json:"id"`
	Region string `json:"region"`
	Arch   string `json:"arch"`
}

func LookupAMI(ctx context.Context, region, arch, version string) (string, error) {
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	url := fmt.Sprintf("https://github.com/siderolabs/talos/releases/download/%s/cloud-images.json", version)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch cloud-images.json: %s", resp.Status)
	}

	var images []CloudImage
	if err := json.NewDecoder(resp.Body).Decode(&images); err != nil {
		return "", err
	}

	for _, img := range images {
		if img.Region == region && img.Arch == arch {
			return img.ID, nil
		}
	}

	return "", fmt.Errorf("AMI not found for region %s and arch %s", region, arch)
}
