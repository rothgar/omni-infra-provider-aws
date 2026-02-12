package main

import (
	"context"
	_ "embed"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/siderolabs/omni/client/pkg/client"
	"github.com/siderolabs/omni/client/pkg/infra"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/siderolabs/omni-infra-provider-aws/internal/pkg/provider"
	"github.com/siderolabs/omni-infra-provider-aws/internal/pkg/provider/meta"
)

//go:embed data/schema.json
var schema string

//go:embed data/icon.svg
var icon []byte

var rootCmd = &cobra.Command{
	Use:          "provider",
	Short:        "AWS Omni infrastructure provider",
	Long:         `Connects to Omni as an infra provider and manages EC2 instances`,
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, _ []string) error {
		loggerConfig := zap.NewProductionConfig()
		logger, err := loggerConfig.Build(zap.AddStacktrace(zapcore.ErrorLevel))
		if err != nil {
			return fmt.Errorf("failed to create logger: %w", err)
		}

		// Check environment variable at runtime if flag is not set
		if cfg.omniAPIEndpoint == "" {
			cfg.omniAPIEndpoint = os.Getenv("OMNI_ENDPOINT")
		}

		if cfg.omniAPIEndpoint == "" {
			return fmt.Errorf("omni-api-endpoint is not set (provide via --omni-api-endpoint flag or OMNI_ENDPOINT env var)")
		}

		ctx := cmd.Context()

		// Check environment variables at runtime if flags are not set
		if cfg.awsProfile == "" {
			cfg.awsProfile = os.Getenv("AWS_PROFILE")
		}
		if cfg.awsRegion == "" {
			cfg.awsRegion = os.Getenv("AWS_REGION")
		}

		var opts []func(*config.LoadOptions) error
		if cfg.awsProfile != "" {
			opts = append(opts, config.WithSharedConfigProfile(cfg.awsProfile))
		}

		awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
		if err != nil {
			return fmt.Errorf("failed to load AWS config: %w", err)
		}

		if cfg.awsRegion != "" {
			awsCfg.Region = cfg.awsRegion
		}

		if awsCfg.Region == "" {
			return fmt.Errorf("AWS region is not set (provide via --aws-region flag or AWS_REGION env var)")
		}

		ec2Client := ec2.NewFromConfig(awsCfg)
		provisioner := provider.NewProvisioner(ec2Client, awsCfg.Region)

		ip, err := infra.NewProvider(meta.ProviderID, provisioner, infra.ProviderConfig{
			Name:        cfg.providerName,
			Description: cfg.providerDescription,
			Icon:        base64.RawStdEncoding.EncodeToString(icon),
			Schema:      schema,
		})
		if err != nil {
			return fmt.Errorf("failed to create infra provider: %w", err)
		}

		logger.Info("starting infra provider", zap.String("region", awsCfg.Region))

		clientOptions := []client.Option{
			client.WithInsecureSkipTLSVerify(cfg.insecureSkipVerify),
		}

		// Check environment variable at runtime if flag is not set
		if cfg.serviceAccountKey == "" {
			cfg.serviceAccountKey = os.Getenv("OMNI_SERVICE_ACCOUNT_KEY")
		}

		if cfg.serviceAccountKey != "" {
			clientOptions = append(clientOptions, client.WithServiceAccount(cfg.serviceAccountKey))
		} else if cfg.serviceAccountFile != "" {
			key, err := os.ReadFile(cfg.serviceAccountFile)
			if err != nil {
				return fmt.Errorf("failed to read service account file: %w", err)
			}
			clientOptions = append(clientOptions, client.WithServiceAccount(string(key)))
		} else {
			// Try to fetch from IMDS UserData
			imdsClient := imds.NewFromConfig(awsCfg)
			out, err := imdsClient.GetUserData(ctx, &imds.GetUserDataInput{})
			if err == nil {
				defer out.Content.Close()
				content, err := io.ReadAll(out.Content)
				if err == nil && len(content) > 0 {
					clientOptions = append(clientOptions, client.WithServiceAccount(string(content)))
					logger.Info("using service account from IMDS userdata")
				}
			}
		}

		return ip.Run(ctx, logger, infra.WithOmniEndpoint(cfg.omniAPIEndpoint), infra.WithClientOptions(
			clientOptions...,
		), infra.WithEncodeRequestIDsIntoTokens())
	},
}

var cfg struct {
	omniAPIEndpoint     string
	serviceAccountKey   string
	serviceAccountFile  string
	providerName        string
	providerDescription string
	awsRegion           string
	awsProfile          string
	insecureSkipVerify  bool
}

func main() {
	if err := app(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func app() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGHUP, syscall.SIGTERM)
	defer cancel()

	return rootCmd.ExecuteContext(ctx)
}

func init() {
	rootCmd.Flags().StringVar(&cfg.omniAPIEndpoint, "omni-api-endpoint", os.Getenv("OMNI_ENDPOINT"),
		"Omni API endpoint (defaults to OMNI_ENDPOINT env var)")
	rootCmd.Flags().StringVar(&meta.ProviderID, "id", meta.ProviderID, "the id of the infra provider")
	rootCmd.Flags().StringVar(&cfg.serviceAccountKey, "omni-service-account-key", os.Getenv("OMNI_SERVICE_ACCOUNT_KEY"),
		"Omni service account key (defaults to OMNI_SERVICE_ACCOUNT_KEY env var)")
	rootCmd.Flags().StringVar(&cfg.serviceAccountFile, "omni-service-account-file", "", "Path to Omni service account key file")
	rootCmd.Flags().StringVar(&cfg.providerName, "provider-name", "AWS", "provider name as it appears in Omni")
	rootCmd.Flags().StringVar(&cfg.providerDescription, "provider-description", "AWS infrastructure provider", "Provider description as it appears in Omni")
	rootCmd.Flags().StringVar(&cfg.awsRegion, "aws-region", os.Getenv("AWS_REGION"),
		"AWS region (defaults to AWS_REGION env var)")
	rootCmd.Flags().StringVar(&cfg.awsProfile, "aws-profile", os.Getenv("AWS_PROFILE"),
		"AWS profile (defaults to AWS_PROFILE env var)")
	rootCmd.Flags().BoolVar(&cfg.insecureSkipVerify, "insecure-skip-verify", false, "ignores untrusted certs on Omni side")
}
