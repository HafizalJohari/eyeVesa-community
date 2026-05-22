# eyeVesa on Google Cloud Platform
# Cloud Run + Cloud SQL (PostgreSQL + pgvector) + Artifact Registry
#
# Architecture:
#   Cloud SQL (PostgreSQL 16 + pgvector) <- Cloud Run (gateway-control + gateway-core + resource-adapter)
#   Cloud Run services behind a Load Balancer with HTTPS
#   OPA embedded in gateway-control (no separate OPA service needed)
#
# Prerequisites:
#   1. gcloud CLI installed and authenticated
#   2. Terraform >= 1.5
#   3. Enable these APIs: run, sqladmin, artifactregistry, cloudbuild, secretmanager, compute

terraform {
  required_version = ">= 1.5"
  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 5.0"
    }
    random = {
      source  = "hashicorp/random"
      version = "~> 3.6"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

provider "google" {
  project = var.project_id
  region  = var.region
}

variable "project_id" {
  description = "GCP project ID"
  type        = string
}

variable "region" {
  description = "GCP region"
  type        = string
  default     = "asia-southeast1"
}

variable "environment" {
  description = "Environment name (staging, production)"
  type        = string
  default     = "production"
}

variable "domain" {
  description = "Domain for the gateway (e.g. gateway.eyevesa.ai)"
  type        = string
  default     = "gateway.eyevesa.ai"
}

variable "db_password" {
  description = "Cloud SQL postgres password"
  type        = string
  sensitive   = true
}

variable "jwt_secret" {
  description = "JWT signing secret"
  type        = string
  sensitive   = true
}

variable "gateway_ed25519_key" {
  description = "Ed25519 signing key for gateway (base64-encoded)"
  type        = string
  sensitive   = true
}

variable "db_tier" {
  description = "Cloud SQL machine tier"
  type        = string
  default     = "db-custom-1-3840"
}

variable "db_availability_type" {
  description = "Cloud SQL availability type"
  type        = string
  default     = "REGIONAL"
}

variable "db_deletion_protection" {
  description = "Protect Cloud SQL from accidental deletion"
  type        = bool
  default     = true
}

variable "enable_cloud_sql_public_ip" {
  description = "Enable public IPv4 on Cloud SQL. Keep false for production."
  type        = bool
  default     = false
}

# ──────────────────────────────────────────────────────────────────
# VPC + Networking
# ──────────────────────────────────────────────────────────────────

resource "google_compute_network" "eyevesa" {
  name                    = "eyevesa-${var.environment}"
  auto_create_subnetworks = false
}

resource "google_compute_global_address" "private_service_access" {
  name          = "eyevesa-private-service-access-${var.environment}"
  purpose       = "VPC_PEERING"
  address_type  = "INTERNAL"
  prefix_length = 16
  network       = google_compute_network.eyevesa.id
}

resource "google_service_networking_connection" "private_service_access" {
  network                 = google_compute_network.eyevesa.id
  service                 = "servicenetworking.googleapis.com"
  reserved_peering_ranges = [google_compute_global_address.private_service_access.name]
}

resource "google_compute_subnetwork" "eyevesa" {
  name          = "eyevesa-${var.environment}"
  network       = google_compute_network.eyevesa.id
  region        = var.region
  ip_cidr_range = "10.0.0.0/24"

  secondary_ip_range {
    range_name    = "pod-range"
    ip_cidr_range = "10.1.0.0/20"
  }
  secondary_ip_range {
    range_name    = "service-range"
    ip_cidr_range = "10.2.0.0/20"
  }
}

resource "google_compute_global_address" "eyevesa" {
  name         = "eyevesa-${var.environment}"
  address_type = "EXTERNAL"
  ip_version   = "IPV4"
}

# VPC connector for Cloud Run -> Cloud SQL
resource "google_vpc_access_connector" "eyevesa" {
  name          = "eyevesa-${var.environment}"
  region        = var.region
  network       = google_compute_network.eyevesa.id
  ip_cidr_range = "10.8.0.0/28"
  min_instances = 2
  max_instances = 3
}

# ──────────────────────────────────────────────────────────────────
# Cloud SQL — PostgreSQL 16 (existing eyevesa-db)
# ──────────────────────────────────────────────────────────────────

resource "google_sql_database_instance" "eyevesa" {
  name             = "eyevesa-db"
  database_version = "POSTGRES_16"
  region           = var.region

  depends_on = [google_service_networking_connection.private_service_access]

  settings {
    tier = var.db_tier

    disk_size         = 10
    disk_type         = "PD_SSD"
    pricing_plan      = "PER_USE"
    availability_type = var.db_availability_type

    ip_configuration {
      ipv4_enabled    = var.enable_cloud_sql_public_ip
      private_network = google_compute_network.eyevesa.id
    }

    database_flags {
      name  = "cloudsql.enable_pgaudit"
      value = "off"
    }
  }

  deletion_protection = var.db_deletion_protection
}

resource "random_id" "db_name" {
  byte_length = 4
}

resource "google_sql_database" "eyevesa" {
  name     = "agentid"
  instance = google_sql_database_instance.eyevesa.name
}

resource "google_sql_user" "eyevesa" {
  name     = "agentid"
  instance = google_sql_database_instance.eyevesa.name
  password = var.db_password
}

# Enable pgvector extension
resource "null_resource" "enable_pgvector" {
  depends_on = [google_sql_user.eyevesa]

  provisioner "local-exec" {
    command = <<-EOT
      cloud-sql-proxy ${google_sql_database_instance.eyevesa.connection_name} --port 5432 &
      PROXY_PID=$!
      sleep 5
      PGPASSWORD=${var.db_password} psql -h 127.0.0.1 -p 5432 -U agentid -d agentid -c "CREATE EXTENSION IF NOT EXISTS vector;" || echo "WARN: pgvector extension creation failed, may need manual setup"
      kill $PROXY_PID 2>/dev/null || true
    EOT
  }

  triggers = {
    instance = google_sql_database_instance.eyevesa.name
  }
}

# ──────────────────────────────────────────────────────────────────
# Secret Manager
# ──────────────────────────────────────────────────────────────────

resource "google_secret_manager_secret" "db_password" {
  secret_id = "eyevesa-db-password-${var.environment}"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "db_password" {
  secret      = google_secret_manager_secret.db_password.id
  secret_data = var.db_password
}

resource "google_secret_manager_secret" "database_url" {
  secret_id = "eyevesa-database-url-${var.environment}"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "database_url" {
  secret      = google_secret_manager_secret.database_url.id
  secret_data = "postgres://agentid:${var.db_password}@/${google_sql_database_instance.eyevesa.name}?host=/cloudsql/${google_sql_database_instance.eyevesa.connection_name}&dbname=agentid&sslmode=disable"
}

resource "google_secret_manager_secret" "jwt_secret" {
  secret_id = "eyevesa-jwt-secret-${var.environment}"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "jwt_secret" {
  secret      = google_secret_manager_secret.jwt_secret.id
  secret_data = var.jwt_secret
}

resource "google_secret_manager_secret" "gateway_key" {
  secret_id = "eyevesa-gateway-ed25519-key-${var.environment}"
  replication {
    auto {}
  }
}

resource "google_secret_manager_secret_version" "gateway_key" {
  secret      = google_secret_manager_secret.gateway_key.id
  secret_data = var.gateway_ed25519_key
}

# ──────────────────────────────────────────────────────────────────
# Artifact Registry
# ──────────────────────────────────────────────────────────────────

resource "google_artifact_registry_repository" "eyevesa" {
  location      = var.region
  repository_id = "eyevesa-${var.environment}"
  format        = "DOCKER"
  description   = "eyeVesa container images"
}

# ──────────────────────────────────────────────────────────────────
# Service Accounts
# ──────────────────────────────────────────────────────────────────

resource "google_service_account" "gateway_control" {
  account_id   = "eyevesa-control-${var.environment}"
  display_name = "eyeVesa gateway-control ${var.environment}"
}

resource "google_secret_manager_secret_iam_member" "gateway_control_database_url" {
  secret_id = google_secret_manager_secret.database_url.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.gateway_control.email}"
}

resource "google_secret_manager_secret_iam_member" "gateway_control_jwt_secret" {
  secret_id = google_secret_manager_secret.jwt_secret.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.gateway_control.email}"
}

resource "google_secret_manager_secret_iam_member" "gateway_control_gateway_key" {
  secret_id = google_secret_manager_secret.gateway_key.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = "serviceAccount:${google_service_account.gateway_control.email}"
}

# ──────────────────────────────────────────────────────────────────
# Cloud Run — gateway-control (Go)
# ──────────────────────────────────────────────────────────────────

data "google_iam_policy" "cloudrun_invoker" {
  binding {
    role = "roles/run.invoker"
    members = [
      "allUsers",
    ]
  }
}

resource "google_cloud_run_v2_service" "gateway_control" {
  name     = "gateway-control-${var.environment}"
  location = var.region

  template {
    service_account = google_service_account.gateway_control.email

    containers {
      name  = "gateway-control"
      image = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.eyevesa.repository_id}/gateway-control:latest"

      command = ["/bin/sh", "-c", "cp /secrets/gateway-ed25519-key /tmp/agentid-gateway-ed25519.key 2>/dev/null; cp /secrets/ptv-ecdsa-key /tmp/agentid-ptv-ecdsa.key 2>/dev/null; /usr/local/bin/agentid-control"]

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = "2000m"
          memory = "512Mi"
        }
      }

      env {
        name = "DATABASE_URL"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.database_url.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "AUTH_ENABLED"
        value = "true"
      }
      env {
        name = "JWT_SECRET"
        value_source {
          secret_key_ref {
            secret  = google_secret_manager_secret.jwt_secret.secret_id
            version = "latest"
          }
        }
      }
      env {
        name  = "OPA_ENDPOINT"
        value = ""
      }
      env {
        name  = "POLICY_DIR"
        value = "/policies"
      }
      env {
        name  = "GATEWAY_KEY_PATH"
        value = "/tmp/agentid-gateway-ed25519.key"
      }
      env {
        name  = "PTV_KEY_PATH"
        value = "/tmp/agentid-ptv-ecdsa.key"
      }

      volume_mounts {
        name       = "cloudsql"
        mount_path = "/cloudsql"
      }
      volume_mounts {
        name       = "secrets"
        mount_path = "/secrets"
      }
    }

    vpc_access {
      connector = google_vpc_access_connector.eyevesa.id
      egress    = "PRIVATE_RANGES_ONLY"
    }

    volumes {
      name = "cloudsql"
      cloud_sql_instance {
        instances = [google_sql_database_instance.eyevesa.connection_name]
      }
    }
    volumes {
      name = "secrets"
      secret {
        secret       = google_secret_manager_secret.gateway_key.secret_id
        default_mode = 256
        items {
          version = "latest"
          path    = "gateway-ed25519-key"
        }
      }
    }
  }

  depends_on = [
    google_sql_database_instance.eyevesa,
    google_secret_manager_secret_iam_member.gateway_control_database_url,
    google_secret_manager_secret_iam_member.gateway_control_jwt_secret,
    google_secret_manager_secret_iam_member.gateway_control_gateway_key,
  ]
}

resource "google_cloud_run_service_iam_policy" "gateway_control_invoker" {
  service     = google_cloud_run_v2_service.gateway_control.name
  location    = var.region
  policy_data = data.google_iam_policy.cloudrun_invoker.policy_data
}

# ──────────────────────────────────────────────────────────────────
# Cloud Run — gateway-core (Rust)
# ──────────────────────────────────────────────────────────────────

resource "google_cloud_run_v2_service" "gateway_core" {
  name     = "gateway-core-${var.environment}"
  location = var.region

  template {
    containers {
      name  = "gateway-core"
      image = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.eyevesa.repository_id}/gateway-core:latest"

      ports {
        container_port = 9443
      }

      startup_probe {
        initial_delay_seconds = 10
        timeout_seconds       = 5
        period_seconds        = 10
        failure_threshold     = 12
        tcp_socket {
          port = 9443
        }
      }

      resources {
        limits = {
          cpu    = "1000m"
          memory = "512Mi"
        }
      }

      env {
        name  = "CONTROL_PLANE_ADDR"
        value = google_cloud_run_v2_service.gateway_control.uri
      }
      env {
        name  = "CONTROL_PLANE_HTTP_ADDR"
        value = google_cloud_run_v2_service.gateway_control.uri
      }
      env {
        name  = "GATEWAY_MODE"
        value = "plaintext"
      }
      env {
        name  = "RUST_LOG"
        value = "info"
      }
    }

    vpc_access {
      connector = google_vpc_access_connector.eyevesa.id
      egress    = "PRIVATE_RANGES_ONLY"
    }
  }
}

resource "google_cloud_run_service_iam_policy" "gateway_core_invoker" {
  service     = google_cloud_run_v2_service.gateway_core.name
  location    = var.region
  policy_data = data.google_iam_policy.cloudrun_invoker.policy_data
}

# ──────────────────────────────────────────────────────────────────
# Cloud Run — resource-adapter (Go)
# ──────────────────────────────────────────────────────────────────

resource "google_cloud_run_v2_service" "resource_adapter" {
  name     = "resource-adapter-${var.environment}"
  location = var.region

  template {
    containers {
      name  = "resource-adapter"
      image = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.eyevesa.repository_id}/resource-adapter:latest"

      ports {
        container_port = 8443
      }

      resources {
        limits = {
          cpu    = "1000m"
          memory = "512Mi"
        }
      }

      env {
        name  = "RESOURCE_NAME"
        value = "enterprise-resource"
      }
      env {
        name  = "GATEWAY_ENDPOINT"
        value = google_cloud_run_v2_service.gateway_core.uri
      }
    }

    vpc_access {
      connector = google_vpc_access_connector.eyevesa.id
      egress    = "PRIVATE_RANGES_ONLY"
    }
  }
}

# ──────────────────────────────────────────────────────────────────
# HTTPS Load Balancer
# ──────────────────────────────────────────────────────────────────

resource "google_compute_ssl_certificate" "eyevesa" {
  name        = "eyevesa-${var.environment}"
  private_key = tls_private_key.eyevesa.private_key_pem
  certificate = tls_self_signed_cert.eyevesa.cert_pem
}

resource "tls_private_key" "eyevesa" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

resource "tls_self_signed_cert" "eyevesa" {
  private_key_pem = tls_private_key.eyevesa.private_key_pem
  subject {
    common_name  = var.domain
    organization = "eyeVesa"
  }
  validity_period_hours = 8760

  dns_names = [var.domain]

  allowed_uses = [
    "key_encipherment",
    "digital_signature",
    "server_auth",
  ]
}

resource "google_compute_backend_bucket" "eyevesa" {
  name        = "eyevesa-backend-${var.environment}"
  bucket_name = google_storage_bucket.eyevesa_static.name
  enable_cdn  = false
}

resource "google_compute_url_map" "eyevesa" {
  name            = "eyevesa-${var.environment}"
  default_service = google_compute_backend_service.eyevesa_gateway.self_link
}

resource "google_compute_region_network_endpoint_group" "eyevesa_gateway" {
  name                  = "eyevesa-gateway-${var.environment}"
  region                = var.region
  network_endpoint_type = "SERVERLESS"
  cloud_run {
    service = google_cloud_run_v2_service.gateway_core.name
  }
}

resource "google_compute_backend_service" "eyevesa_gateway" {
  name        = "eyevesa-gateway-${var.environment}"
  protocol    = "HTTP"
  timeout_sec = 30

  backend {
    group = google_compute_region_network_endpoint_group.eyevesa_gateway.id
  }
}

# For production, use managed SSL cert instead of self-signed:
# resource "google_compute_managed_ssl_certificate" "eyevesa" {
#   name = "eyevesa-${var.environment}"
#   managed {
#     domains = [var.domain]
#   }
# }

resource "google_storage_bucket" "eyevesa_static" {
  name     = "eyevesa-static-${var.environment}-${random_id.db_name.hex}"
  location = var.region
}

# ──────────────────────────────────────────────────────────────────
# Outputs
# ──────────────────────────────────────────────────────────────────

output "gateway_core_url" {
  description = "URL of the gateway-core Cloud Run service"
  value       = google_cloud_run_v2_service.gateway_core.uri
}

output "gateway_control_url" {
  description = "URL of the gateway-control Cloud Run service"
  value       = google_cloud_run_v2_service.gateway_control.uri
}

output "resource_adapter_url" {
  description = "URL of the resource-adapter Cloud Run service"
  value       = google_cloud_run_v2_service.resource_adapter.uri
}

output "cloud_sql_connection" {
  description = "Cloud SQL instance connection name"
  value       = google_sql_database_instance.eyevesa.connection_name
}

output "artifact_registry_url" {
  description = "Artifact Registry repository URL"
  value       = "${var.region}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.eyevesa.repository_id}"
}

output "vpc_connector_id" {
  description = "VPC Access connector ID"
  value       = google_vpc_access_connector.eyevesa.id
}
