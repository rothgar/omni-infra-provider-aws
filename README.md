# Omni AWS Infrastructure Provider

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

# Add egress rules for IPv4 and IPv6
aws ec2 authorize-security-group-egress \
  --group-id $SG_ID \
  --ip-permissions IpProtocol=-1,IpRanges='[{CidrIp=0.0.0.0/0}]'

aws ec2 authorize-security-group-egress \
  --group-id $SG_ID \
  --ip-permissions IpProtocol=-1,Ipv6Ranges='[{CidrIpv6=::/0}]'
```

### 2. Create Infrastructure Provider and Machine Class

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

Create a machine class for the nodes.

```bash
cat <<EOF > machine-class.yaml
metadata:
  namespace: default
  type: MachineClass.omni.sidero.dev
  id: aws
spec:
  autoprovision:
    providerid: aws
    grpctunnel: 0
    providerdata:
      volume_size: 20
      instance_type: t3.medium
      securite_gorup_ids:
        - $SG_ID
      arch: amd64
      subnet_id: ''
      subnet_ids:
        - $SUBNET_1
        - $SUBNET_2
        - $SUBNET_3
EOF
```

Apply the machine class

```bash
omnictl apply -f machine-class.yaml
```

### 2. Deploy Infrastructure Provider

#### Run locally with Docker

```bash
docker run -d \
  --name omni-infra-provider-aws \
  --restart unless-stopped \
  -e OMNI_ENDPOINT \
  -e OMNI_SERVICE_ACCOUNT_KEY \
  -e AWS_REGION \
  -e AWS_ACCESS_KEY_ID=your-access-key \
  -e AWS_SECRET_ACCESS_KEY=your-secret-key \
  rothgar/omni-infra-provider-aws:latest
```

**Alternative: Use AWS credentials from host**

```bash
docker run -d \
  --name omni-infra-provider-aws \
  --restart unless-stopped \
  -v ~/.aws:$HOME/.aws:ro \
  -e OMNI_ENDPOINT \
  -e OMNI_SERVICE_ACCOUNT_KEY \
  -e AWS_REGION \
  -e AWS_PROFILE=default \
  rothgar/omni-infra-provider-aws:latest
```

#### Deploy to EC2

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
AMI_ID=$(aws ec2 describe-images \
  --region $AWS_REGION \
  --owners 075585003325 \
  --filters "Name=name,Values=Flatcar-stable-*-hvm" \
  --query 'sort_by(Images, &CreationDate)[-1].ImageId' \
  --output text)

# Create EC2 instance with user-data
aws ec2 run-instances \
  --region $AWS_REGION \
  --image-id $AMI_ID \
  --instance-type t3.micro \
  --subnet-id $SUBNET_1 \
  --security-group-ids $SG_ID \
  --iam-instance-profile Name=OmniInfraProviderProfile \
  --user-data "$(cat <<EOF
#!/bin/bash
set -e

docker run -d \
  --name omni-infra-provider-aws \
  --restart unless-stopped \
  -e OMNI_ENDPOINT=$OMNI_ENDPOINT \
  -e OMNI_SERVICE_ACCOUNT_KEY=$OMNI_SERVICE_ACCOUNT_KEY \
  -e AWS_REGION=$AWS_REGION \
  rothgar/omni-infra-provider-aws:latest
EOF
)" \
  --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=omni-infra-provider-aws}]"
```

## Available Parameters

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `instance_type` | string | ✅ Yes | EC2 instance type (e.g., t3.medium) |
| `subnet_id` | string | No | Single subnet ID |
| `subnet_ids` | array | No | Multiple subnet IDs for HA |
| `security_group_ids` | array | Conditional* | Security group IDs |
| `volume_size` | integer | No | Root volume size in GB (default: 8) |
| `arch` | string | No | Architecture: `amd64` or `arm64` (default: amd64) |


## Support

- GitHub Issues: https://github.com/siderolabs/omni-infra-provider-aws/issues
- Omni Documentation: https://www.siderolabs.com/platform/saas-for-kubernetes/
- Talos Documentation: https://www.talos.dev/

