# Omni AWS Infrastructure Provider

:warning: This is not an official Siderolabs provider. Only run this if you liking being woken up at 2:00 AM. :warning:

An infrastructure provider for [Omni](https://www.siderolabs.com/platform/saas-for-kubernetes/) that provisions and manages Talos Linux machines on AWS EC2.

## Quick Start

### 1. Create VPC and subnets with IPv6 support

Create a VPC with IPv6 support and subnets across multiple availability zones:

```bash
# Create VPC with IPv4 CIDR
VPC_ID=$(aws ec2 create-vpc \
  --cidr-block 10.0.0.0/16 \
  --amazon-provided-ipv6-cidr-block \
  --tag-specifications 'ResourceType=vpc,Tags=[{Key=Name,Value=omni-vpc}]' \
  --query 'Vpc.VpcId' --output text)

aws ec2 modify-vpc-attribute --vpc-id $VPC_ID --enable-dns-support
aws ec2 modify-vpc-attribute --vpc-id $VPC_ID --enable-dns-hostnames

# Associate IPv6 CIDR block with VPC
IPV6_CIDR=$(aws ec2 describe-vpcs \
  --vpc-ids $VPC_ID \
  --query 'Vpcs[0].Ipv6CidrBlockAssociationSet[0].Ipv6CidrBlock' --output text)
```

Divide your IPV6_CIDR into 3 separate variables. Here is an example.

```bash
PREFIX=$(echo "$IPV6_CIDR" | cut -d':' -f1-3)
BASE_HEX=$(echo "$IPV6_CIDR" | cut -d':' -f4 | cut -d'/' -f1)
BASE_DEC=$((16#$BASE_HEX))

IPV6_SUBNET_A="${PREFIX}:$(printf "%x" $((BASE_DEC + 0)))::/64"
IPV6_SUBNET_B="${PREFIX}:$(printf "%x" $((BASE_DEC + 1)))::/64"
IPV6_SUBNET_C="${PREFIX}:$(printf "%x" $((BASE_DEC + 2)))::/64"
```

```bash
# Create Internet Gateway
IGW_ID=$(aws ec2 create-internet-gateway \
  --tag-specifications 'ResourceType=internet-gateway,Tags=[{Key=Name,Value=omni-igw}]' \
  --query 'InternetGateway.InternetGatewayId' --output text)

# Attach IGW to VPC
aws ec2 attach-internet-gateway \
  --vpc-id $VPC_ID \
  --internet-gateway-id $IGW_ID

# Create subnets in different AZs
SUBNET_1=$(aws ec2 create-subnet \
  --vpc-id $VPC_ID \
  --cidr-block 10.0.0.0/20 \
  --ipv6-cidr-block $IPV6_SUBNET_A \
  --availability-zone us-west-2a \
  --tag-specifications 'ResourceType=subnet,Tags=[{Key=Name,Value=omni-subnet-2a}]' \
  --query 'Subnet.SubnetId' --output text)

SUBNET_2=$(aws ec2 create-subnet \
  --vpc-id $VPC_ID \
  --cidr-block 10.0.16.0/20 \
  --ipv6-cidr-block $IPV6_SUBNET_B \
  --availability-zone us-west-2b \
  --tag-specifications 'ResourceType=subnet,Tags=[{Key=Name,Value=omni-subnet-2b}]' \
  --query 'Subnet.SubnetId' --output text)

SUBNET_3=$(aws ec2 create-subnet \
  --vpc-id $VPC_ID \
  --cidr-block 10.0.32.0/20 \
  --ipv6-cidr-block $IPV6_SUBNET_C \
  --availability-zone us-west-2c \
  --tag-specifications 'ResourceType=subnet,Tags=[{Key=Name,Value=omni-subnet-2c}]' \
  --query 'Subnet.SubnetId' --output text)

# Enable auto-assign public IPv4 and IPv6 addresses
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_1 --map-public-ip-on-launch
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_2 --map-public-ip-on-launch
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_3 --map-public-ip-on-launch
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_1 --assign-ipv6-address-on-creation
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_2 --assign-ipv6-address-on-creation
aws ec2 modify-subnet-attribute --subnet-id $SUBNET_3 --assign-ipv6-address-on-creation

# Get main route table
RTB_ID=$(aws ec2 describe-route-tables \
  --filters "Name=vpc-id,Values=$VPC_ID" "Name=association.main,Values=true" \
  --query 'RouteTables[0].RouteTableId' --output text)

# Add routes to Internet Gateway
aws ec2 create-route \
  --route-table-id $RTB_ID \
  --destination-cidr-block 0.0.0.0/0 \
  --gateway-id $IGW_ID

aws ec2 create-route \
  --route-table-id $RTB_ID \
  --destination-ipv6-cidr-block ::/0 \
  --gateway-id $IGW_ID

# Create security group
SG_ID=$(aws ec2 create-security-group \
  --group-name omni-talos \
  --description "Security group for Omni Talos nodes" \
  --vpc-id $VPC_ID \
  --tag-specifications 'ResourceType=security-group,Tags=[{Key=Name,Value=omni-talos}]' \
  --query 'GroupId' --output text)

# Add security group rules
# SSH access from your current IP (for troubleshooting)
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID \
  --protocol tcp \
  --port 22 \
  --cidr $(curl -s ifconfig.me)/32

# Allow all traffic within the security group (for node-to-node communication)
aws ec2 authorize-security-group-ingress \
  --group-id $SG_ID \
  --protocol -1 \
  --source-group $SG_ID

# Allow all outbound traffic (default for new security groups)
```

### 2. Deploy Infrastructure Provider

Create the infrastructure provider via `omnictl`

```bash
omnictl infraprovider create aws
```
This will print output like:

```bash
OMNI_ENDPOINT=https://omni...
OMNI_SERVICE_ACCOUNT_KEY=elashecidgcegiDEDTNE...
```
Export these environment variables into your shell and save them to a file.

```bash
echo "export OMNI_ENDPOINT=$OMNI_ENDPOINT" > .env
echo "export OMNI_SERVICE_ACCOUNT_KEY=$OMNI_SERVICE_ACCOUNT_KEY" >> .env
```

Source the file

```bash
source .env
```

#### Run locally with Docker

This mounts your local AWS credentials into the container for AWS authentication.

```bash
docker run -d \
  --name omni-infra-provider-aws \
  --restart unless-stopped \
  -v $HOME/.aws:/home/omni/.aws:ro \
  -e OMNI_ENDPOINT \
  -e OMNI_SERVICE_ACCOUNT_KEY \
  -e AWS_PROFILE=default \
  ghcr.io/rothgar/omni-infra-provider-aws:latest
```

<details>
<summary><h4>Deploy to EC2 (Optional)</h4></summary>

**Prerequisites: Create IAM Role**

First, create an IAM role with the required permissions:

```bash
# Create IAM policy with minimal permissions
cat > omni-provider-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "ec2:RunInstances",
        "ec2:DescribeInstances",
        "ec2:DescribeInstanceStatus",
        "ec2:DescribeImages",
        "ec2:DescribeSubnets",
        "ec2:DescribeSecurityGroups",
        "ec2:DescribeKeyPairs",
        "ec2:CreateTags"
      ],
      "Resource": "*"
    },
    {
      "Effect": "Allow",
      "Action": [
        "ec2:TerminateInstances"
      ],
      "Resource": "arn:aws:ec2:*:*:instance/*",
      "Condition": {
        "StringEquals": {
          "ec2:ResourceTag/ManagedBy": "omni-infra-provider"
        }
      }
    }
  ]
}
EOF

# Create the IAM policy
aws iam create-policy \
  --policy-name OmniInfraProviderPolicy \
  --policy-document file://omni-provider-policy.json

# Create IAM role for EC2
cat > trust-policy.json <<'EOF'
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Principal": {
        "Service": "ec2.amazonaws.com"
      },
      "Action": "sts:AssumeRole"
    }
  ]
}
EOF

aws iam create-role \
  --role-name OmniInfraProviderRole \
  --assume-role-policy-document file://trust-policy.json

# Attach policy to role
ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
aws iam attach-role-policy \
  --role-name OmniInfraProviderRole \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/OmniInfraProviderPolicy

# Create instance profile
aws iam create-instance-profile \
  --instance-profile-name OmniInfraProviderProfile

aws iam add-role-to-instance-profile \
  --instance-profile-name OmniInfraProviderProfile \
  --role-name OmniInfraProviderRole

# Cleanup policy files
rm omni-provider-policy.json trust-policy.json
```

**Create EC2 Instance**

```bash
# Create or use existing SSH key pair (optional, for troubleshooting)
KEY_NAME="omni-provider-key"
aws ec2 create-key-pair --key-name $KEY_NAME --query 'KeyMaterial' --output text > ~/.ssh/$KEY_NAME.pem
chmod 400 ~/.ssh/$KEY_NAME.pem

# Find latest Flatcar AMI
AMI_ID=$(aws ec2 describe-images \
  --owners 075585003325 \
  --filters "Name=name,Values=Flatcar-stable-*-hvm" "Name=architecture,Values=x86_64" \
  --query 'sort_by(Images, &CreationDate)[-1].ImageId' \
  --output text)

# Create EC2 instance with user-data
INSTANCE_ID=$(aws ec2 run-instances \
  --image-id $AMI_ID \
  --instance-type t3.micro \
  --subnet-id $SUBNET_1 \
  --security-group-ids $SG_ID \
  --key-name $KEY_NAME \
  --iam-instance-profile Name=OmniInfraProviderProfile \
  --user-data "$(cat <<EOF
#!/bin/bash
set -e

# Get AWS region from instance metadata
TOKEN=\$(curl -s -X PUT "http://169.254.169.254/latest/api/token" -H "X-aws-ec2-metadata-token-ttl-seconds: 21600")
AWS_REGION=\$(curl -s -H "X-aws-ec2-metadata-token: \$TOKEN" http://169.254.169.254/latest/meta-data/placement/region)

docker run -d \
  --name omni-infra-provider-aws \
  --restart unless-stopped \
  -e OMNI_ENDPOINT=$OMNI_ENDPOINT \
  -e OMNI_SERVICE_ACCOUNT_KEY=$OMNI_SERVICE_ACCOUNT_KEY \
  -e AWS_REGION=\$AWS_REGION \
  ghcr.io/rothgar/omni-infra-provider-aws:latest
EOF
)" \
  --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=omni-infra-provider-aws}]" \
  --query 'Instances[0].InstanceId' \
  --output text)

echo "Instance created: $INSTANCE_ID"
```

**View Container Logs**

To view logs from the provider container:

```bash
# Get the instance's public IP
INSTANCE_IP=$(aws ec2 describe-instances \
  --instance-ids $INSTANCE_ID \
  --query 'Reservations[0].Instances[0].PublicIpAddress' \
  --output text)

# SSH into the instance (default user is 'core' for Flatcar)
ssh -i ~/.ssh/$KEY_NAME.pem core@$INSTANCE_IP
```
Read logs

```bash
```
```
# Once connected, view container logs:
docker logs omni-infra-provider-aws

# Follow logs in real-time:
docker logs -f omni-infra-provider-aws

# View last 100 lines:
docker logs --tail 100 omni-infra-provider-aws
```

</details>

### 3. Create Infrastructure Provider and Machine Class


Create a machine class for the nodes.

```bash
cat <<EOF > machine-class.yaml
metadata:
  namespace: default
  type: MachineClasses.omni.sidero.dev
  id: aws
spec:
  autoprovision:
    providerid: aws
    grpctunnel: 0
    providerdata: '{"volume_size":20,"instance_type":"t3.medium","security_group_ids":["$SG_ID"],"arch":"amd64","subnet_ids":["$SUBNET_1","$SUBNET_2","$SUBNET_3"]}'
EOF
```

**Note:** The `providerdata` field must be a JSON-encoded string, not a YAML object.

Apply the machine class. Make sure you run this from your user's Omni credentials and not with the `OMNI_SERVICE_ACCOUNT_KEY`.

```bash
omnictl apply -f machine-class.yaml
```

### 4. Create a Cluster

Create a cluster template file or use the provided example:

```bash
cat <<EOF > cluster-template.yaml
kind: Cluster
name: aws
kubernetes:
  version: v1.34.2
talos:
  version: v1.12.3
---
kind: ControlPlane
machineClass:
  name: aws
  size: 1
---
kind: Workers
machineClass:
  name: aws
  size: 3
EOF
```

Apply the cluster template to create your cluster:

```bash
omnictl cluster template sync -f cluster-template.yaml
```

This will create a cluster named "aws" with 1 control plane node and 3 worker nodes using the machine class you created.

## Available Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `instance_type` | string | ✅ Yes | EC2 instance type (e.g., t3.medium) |
| `subnet_id` | string | No | Single subnet ID |
| `subnet_ids` | array | No | Multiple subnet IDs for HA |
| `security_group_ids` | array | Conditional* | Security group IDs |
| `volume_size` | integer | No | Root volume size in GB (default: 8) |
| `arch` | string | No | Architecture: `amd64` or `arm64` (default: amd64) |


## Cleanup

To remove all AWS resources created in this guide:

```bash
# Stop and remove the provider container (if running locally)
docker stop omni-infra-provider-aws
docker rm omni-infra-provider-aws

# Find and terminate all EC2 instances created by the provider
PROVIDER_INSTANCES=$(aws ec2 describe-instances \
  --filters "Name=tag-key,Values=omni-request-id" "Name=instance-state-name,Values=running,stopped,pending" \
  --query 'Reservations[*].Instances[*].InstanceId' --output text)

if [ -n "$PROVIDER_INSTANCES" ]; then
  echo "Terminating provider-managed instances: $PROVIDER_INSTANCES"
  aws ec2 terminate-instances --instance-ids $PROVIDER_INSTANCES
  aws ec2 wait instance-terminated --instance-ids $PROVIDER_INSTANCES
fi

# Find and terminate the provider EC2 instance (if deployed to EC2)
PROVIDER_HOST=$(aws ec2 describe-instances \
  --filters "Name=tag:Name,Values=omni-infra-provider-aws" "Name=instance-state-name,Values=running,stopped,pending" \
  --query 'Reservations[*].Instances[*].InstanceId' --output text)

if [ -n "$PROVIDER_HOST" ]; then
  echo "Terminating provider host instance: $PROVIDER_HOST"
  aws ec2 terminate-instances --instance-ids $PROVIDER_HOST
  aws ec2 wait instance-terminated --instance-ids $PROVIDER_HOST
fi

# Delete security group
if [ -n "$SG_ID" ]; then
  aws ec2 delete-security-group --group-id $SG_ID
fi

# Delete subnets
for subnet in $SUBNET_1 $SUBNET_2 $SUBNET_3; do
  if [ -n "$subnet" ]; then
    aws ec2 delete-subnet --subnet-id $subnet
  fi
done

# Detach and delete internet gateway
if [ -n "$IGW_ID" ] && [ -n "$VPC_ID" ]; then
  aws ec2 detach-internet-gateway --internet-gateway-id $IGW_ID --vpc-id $VPC_ID
  aws ec2 delete-internet-gateway --internet-gateway-id $IGW_ID
fi

# Delete VPC
if [ -n "$VPC_ID" ]; then
  aws ec2 delete-vpc --vpc-id $VPC_ID
fi

# Delete IAM resources (if created)
aws iam remove-role-from-instance-profile \
  --instance-profile-name OmniInfraProviderProfile \
  --role-name OmniInfraProviderRole 2>/dev/null || true

aws iam delete-instance-profile \
  --instance-profile-name OmniInfraProviderProfile 2>/dev/null || true

ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
aws iam detach-role-policy \
  --role-name OmniInfraProviderRole \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/OmniInfraProviderPolicy 2>/dev/null || true

aws iam delete-role \
  --role-name OmniInfraProviderRole 2>/dev/null || true

aws iam delete-policy \
  --policy-arn arn:aws:iam::${ACCOUNT_ID}:policy/OmniInfraProviderPolicy 2>/dev/null || true

echo "Cleanup complete"
```

**Note:** This assumes you still have the environment variables from the setup section. If you don't, you can find the resources by their Name tags or manually through the AWS console.

## Releases

Container images are automatically built and pushed to GitHub Container Registry when a new tag is created.

### Available Tags

- `ghcr.io/rothgar/omni-infra-provider-aws:latest` - Latest release
- `ghcr.io/rothgar/omni-infra-provider-aws:v1.2.3` - Specific version
- `ghcr.io/rothgar/omni-infra-provider-aws:1.2` - Major.minor version
- `ghcr.io/rothgar/omni-infra-provider-aws:1` - Major version
- `ghcr.io/rothgar/omni-infra-provider-aws:sha-<commit>` - Specific commit SHA

### Creating a Release

To create a new release:

```bash
git tag -a v1.0.0 -m "Release v1.0.0"
git push origin v1.0.0
```

The GitHub Action will automatically build multi-arch images (amd64 and arm64) and push them to GHCR.

## Support

- GitHub Issues: https://github.com/siderolabs/omni-infra-provider-aws/issues
- Omni Documentation: https://www.siderolabs.com/platform/saas-for-kubernetes/
- Talos Documentation: https://www.talos.dev/

