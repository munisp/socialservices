variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "prod"
  
  validation {
    condition     = contains(["dev", "staging", "prod"], var.environment)
    error_message = "Environment must be one of: dev, staging, prod."
  }
}

variable "key_pair_name" {
  description = "Name of the SSH key pair for instance access"
  type        = string
  default     = "social-protection-key"
}

variable "master_count" {
  description = "Number of Kubernetes master nodes (should be odd for etcd quorum)"
  type        = number
  default     = 3
  
  validation {
    condition     = var.master_count >= 1 && var.master_count % 2 == 1
    error_message = "Master count must be an odd number >= 1 for proper etcd quorum."
  }
}

variable "worker_count" {
  description = "Number of Kubernetes worker nodes"
  type        = number
  default     = 5
  
  validation {
    condition     = var.worker_count >= 1
    error_message = "Worker count must be at least 1."
  }
}

variable "storage_count" {
  description = "Number of Ceph storage nodes (minimum 3 for replication)"
  type        = number
  default     = 3
  
  validation {
    condition     = var.storage_count >= 3
    error_message = "Storage count must be at least 3 for Ceph replication."
  }
}

variable "external_network_name" {
  description = "Name of the external network for floating IPs"
  type        = string
  default     = "external"
}

variable "dns_nameservers" {
  description = "DNS nameservers for the subnet"
  type        = list(string)
  default     = ["8.8.8.8", "8.8.4.4"]
}

variable "master_flavor" {
  description = "OpenStack flavor for master nodes"
  type        = string
  default     = "m1.xlarge"
}

variable "worker_flavor" {
  description = "OpenStack flavor for worker nodes"
  type        = string
  default     = "m1.2xlarge"
}

variable "storage_flavor" {
  description = "OpenStack flavor for storage nodes"
  type        = string
  default     = "m1.xlarge"
}

variable "osd_volume_size" {
  description = "Size of OSD volumes in GB"
  type        = number
  default     = 500
}

variable "network_cidr" {
  description = "CIDR for the main network"
  type        = string
  default     = "10.0.0.0/16"
}

variable "pod_cidr" {
  description = "CIDR for Kubernetes pods"
  type        = string
  default     = "10.244.0.0/16"
}

variable "service_cidr" {
  description = "CIDR for Kubernetes services"
  type        = string
  default     = "10.96.0.0/12"
}

variable "kubernetes_version" {
  description = "Kubernetes version to install"
  type        = string
  default     = "1.29.0"
}

variable "ceph_release" {
  description = "Ceph release to install"
  type        = string
  default     = "quincy"
}

variable "tags" {
  description = "Tags to apply to resources"
  type        = map(string)
  default = {
    Project     = "social-protection-platform"
    ManagedBy   = "terraform"
    Environment = "prod"
  }
}
