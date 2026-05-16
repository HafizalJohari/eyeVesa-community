# AgentID Gateway - Terraform Infrastructure
# Southeast Asia (Malaysia/Singapore) deployment target

provider "aws" {
  region = var.aws_region
}

variable "aws_region" {
  default = "ap-southeast-1"
}

variable "environment" {
  default = "production"
}

variable "domain" {
  default = "agentid.dev"
}

# VPC
resource "aws_vpc" "agentid" {
  cidr_block           = "10.0.0.0/16"
  enable_dns_hostnames = true
  enable_dns_support   = true

  tags = {
    Name        = "agentid-vpc"
    Environment = var.environment
  }
}

resource "aws_internet_gateway" "agentid" {
  vpc_id = aws_vpc.agentid.id

  tags = {
    Name = "agentid-igw"
  }
}

resource "aws_subnet" "private" {
  count             = 2
  vpc_id            = aws_vpc.agentid.id
  cidr_block        = "10.0.${count.index + 1}.0/24"
  availability_zone  = element(["ap-southeast-1a", "ap-southeast-1b"], count.index)

  tags = {
    Name = "agentid-private-${count.index + 1}"
  }
}

resource "aws_subnet" "public" {
  count             = 2
  vpc_id            = aws_vpc.agentid.id
  cidr_block        = "10.0.${count.index + 10}.0/24"
  availability_zone  = element(["ap-southeast-1a", "ap-southeast-1b"], count.index)

  tags = {
    Name = "agentid-public-${count.index + 1}"
  }
}

# EKS Cluster
resource "aws_eks_cluster" "agentid" {
  name     = "agentid-${var.environment}"
  role_arn  = aws_iam_role.eks_cluster.arn
  version  = "1.29"

  vpc_config {
    subnet_ids = concat(aws_subnet.private[*].id, aws_subnet.public[*].id)
  }

  tags = {
    Name = "agentid-eks"
  }
}

resource "aws_eks_node_group" "gateway" {
  cluster_name    = aws_eks_cluster.agentid.name
  node_group_name = "gateway"
  node_role_arn   = aws_iam_role.eks_node.arn
  subnet_ids      = aws_subnet.private[*].id

  scaling_config {
    desired_size = 2
    max_size     = 5
    min_size     = 1
  }

  instance_types = ["t3.medium"]
}

# RDS PostgreSQL + pgvector
resource "aws_db_subnet_group" "agentid" {
  name       = "agentid"
  subnet_ids = aws_subnet.private[*].id

  tags = {
    Name = "agentid-db-subnet"
  }
}

resource "aws_security_group" "db" {
  name        = "agentid-db-sg"
  description = "AgentID database security group"
  vpc_id      = aws_vpc.agentid.id

  ingress {
    from_port   = 5432
    to_port     = 5432
    protocol    = "tcp"
    security_groups = [aws_security_group.eks_nodes.id]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_db_instance" "agentid" {
  identifier           = "agentid-${var.environment}"
  engine               = "postgres"
  engine_version       = "16"
  instance_class       = "db.r6g.large"
  allocated_storage     = 100
  storage_encrypted    = true
  db_name              = "agentid"
  username             = "agentid"
  password             = var.db_password
  db_subnet_group_name = aws_db_subnet_group.agentid.name
  vpc_security_group_ids = [aws_security_group.db.id]
  skip_final_snapshot  = false
  final_snapshot_identifier = "agentid-final"
  parameter_group_name = aws_db_parameter_group.pgvector.name
}

resource "aws_db_parameter_group" "pgvector" {
  name   = "agentid-pgvector"
  family = "postgres16"

  parameter {
    name  = "shared_preload_libraries"
    value = "vector"
  }
}

resource "aws_security_group" "eks_nodes" {
  name        = "agentid-eks-nodes-sg"
  description = "EKS node security group"
  vpc_id      = aws_vpc.agentid.id

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    self        = true
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }
}

# ECR Repositories
resource "aws_ecr_repository" "gateway_control" {
  name = "agentid/gateway-control"
}

resource "aws_ecr_repository" "gateway_core" {
  name = "agentid/gateway-core"
}

resource "aws_ecr_repository" "resource_adapter" {
  name = "agentid/resource-adapter"
}

# IAM Roles
resource "aws_iam_role" "eks_cluster" {
  name = "agentid-eks-cluster"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "eks.amazonaws.com"
      }
    }]
  })
}

resource "aws_iam_role" "eks_node" {
  name = "agentid-eks-node"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action = "sts:AssumeRole"
      Effect = "Allow"
      Principal = {
        Service = "ec2.amazonaws.com"
      }
    }]
  })
}

# Secrets Manager for gateway key
resource "aws_secretsmanager_secret" "gateway_key" {
  name                    = "agentid/gateway-ed25519-key"
  recovery_window_in_days = 7
}

resource "aws_secretsmanager_secret" "push_credentials" {
  name                    = "agentid/push-credentials"
  recovery_window_in_days = 7
}

variable "db_password" {
  type      = string
  sensitive = true
}