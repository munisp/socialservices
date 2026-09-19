terraform {
  required_version = ">= 1.0.0"
  
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "~> 1.54.0"
    }
  }
  
  backend "swift" {
    container         = "terraform-state"
    archive_container = "terraform-state-archive"
  }
}

provider "openstack" {
  # Configuration from environment variables:
  # OS_AUTH_URL, OS_PROJECT_NAME, OS_USERNAME, OS_PASSWORD, etc.
}

# Variables
variable "environment" {
  description = "Environment name (dev, staging, prod)"
  type        = string
  default     = "prod"
}

variable "key_pair_name" {
  description = "Name of the SSH key pair"
  type        = string
  default     = "social-protection-key"
}

variable "master_count" {
  description = "Number of Kubernetes master nodes"
  type        = number
  default     = 3
}

variable "worker_count" {
  description = "Number of Kubernetes worker nodes"
  type        = number
  default     = 5
}

variable "storage_count" {
  description = "Number of Ceph storage nodes"
  type        = number
  default     = 3
}

variable "external_network_name" {
  description = "Name of the external network"
  type        = string
  default     = "external"
}

variable "dns_nameservers" {
  description = "DNS nameservers"
  type        = list(string)
  default     = ["8.8.8.8", "8.8.4.4"]
}

# Data sources
data "openstack_images_image_v2" "ubuntu" {
  name        = "Ubuntu-22.04-LTS"
  most_recent = true
}

data "openstack_networking_network_v2" "external" {
  name = var.external_network_name
}

# Network
resource "openstack_networking_network_v2" "main" {
  name           = "social-protection-${var.environment}-network"
  admin_state_up = true
}

resource "openstack_networking_subnet_v2" "main" {
  name            = "social-protection-${var.environment}-subnet"
  network_id      = openstack_networking_network_v2.main.id
  cidr            = "10.0.0.0/16"
  gateway_ip      = "10.0.0.1"
  dns_nameservers = var.dns_nameservers
  
  allocation_pool {
    start = "10.0.1.10"
    end   = "10.0.255.250"
  }
}

resource "openstack_networking_router_v2" "main" {
  name                = "social-protection-${var.environment}-router"
  external_network_id = data.openstack_networking_network_v2.external.id
}

resource "openstack_networking_router_interface_v2" "main" {
  router_id = openstack_networking_router_v2.main.id
  subnet_id = openstack_networking_subnet_v2.main.id
}

# Security Groups
resource "openstack_networking_secgroup_v2" "master" {
  name        = "social-protection-${var.environment}-master-sg"
  description = "Security group for Kubernetes master nodes"
}

resource "openstack_networking_secgroup_rule_v2" "master_ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_rule_v2" "master_api" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 6443
  port_range_max    = 6443
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_rule_v2" "master_etcd" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 2379
  port_range_max    = 2380
  remote_ip_prefix  = "10.0.0.0/16"
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_rule_v2" "master_kubelet" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 10250
  port_range_max    = 10259
  remote_ip_prefix  = "10.0.0.0/16"
  security_group_id = openstack_networking_secgroup_v2.master.id
}

resource "openstack_networking_secgroup_v2" "worker" {
  name        = "social-protection-${var.environment}-worker-sg"
  description = "Security group for Kubernetes worker nodes"
}

resource "openstack_networking_secgroup_rule_v2" "worker_ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

resource "openstack_networking_secgroup_rule_v2" "worker_kubelet" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 10250
  port_range_max    = 10250
  remote_ip_prefix  = "10.0.0.0/16"
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

resource "openstack_networking_secgroup_rule_v2" "worker_nodeport" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.worker.id
}

resource "openstack_networking_secgroup_v2" "storage" {
  name        = "social-protection-${var.environment}-storage-sg"
  description = "Security group for Ceph storage nodes"
}

resource "openstack_networking_secgroup_rule_v2" "storage_ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.storage.id
}

resource "openstack_networking_secgroup_rule_v2" "storage_ceph_mon" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 6789
  port_range_max    = 6789
  remote_ip_prefix  = "10.0.0.0/16"
  security_group_id = openstack_networking_secgroup_v2.storage.id
}

resource "openstack_networking_secgroup_rule_v2" "storage_ceph_osd" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 6800
  port_range_max    = 7300
  remote_ip_prefix  = "10.0.0.0/16"
  security_group_id = openstack_networking_secgroup_v2.storage.id
}

# Load Balancer
resource "openstack_lb_loadbalancer_v2" "api" {
  name          = "social-protection-${var.environment}-api-lb"
  vip_subnet_id = openstack_networking_subnet_v2.main.id
}

resource "openstack_lb_listener_v2" "api" {
  name            = "social-protection-${var.environment}-api-listener"
  loadbalancer_id = openstack_lb_loadbalancer_v2.api.id
  protocol        = "TCP"
  protocol_port   = 6443
}

resource "openstack_lb_pool_v2" "api" {
  name        = "social-protection-${var.environment}-api-pool"
  listener_id = openstack_lb_listener_v2.api.id
  lb_method   = "ROUND_ROBIN"
  protocol    = "TCP"
}

resource "openstack_lb_monitor_v2" "api" {
  pool_id     = openstack_lb_pool_v2.api.id
  type        = "TCP"
  delay       = 5
  timeout     = 5
  max_retries = 3
}

resource "openstack_networking_floatingip_v2" "api" {
  pool = var.external_network_name
}

resource "openstack_networking_floatingip_associate_v2" "api" {
  floating_ip = openstack_networking_floatingip_v2.api.address
  port_id     = openstack_lb_loadbalancer_v2.api.vip_port_id
}

# Master Nodes
resource "openstack_compute_instance_v2" "master" {
  count           = var.master_count
  name            = "social-protection-${var.environment}-master-${count.index}"
  image_id        = data.openstack_images_image_v2.ubuntu.id
  flavor_name     = "m1.xlarge"
  key_pair        = var.key_pair_name
  security_groups = [openstack_networking_secgroup_v2.master.name]

  network {
    uuid = openstack_networking_network_v2.main.id
  }

  user_data = templatefile("${path.module}/templates/master-init.sh", {
    api_lb_ip = openstack_lb_loadbalancer_v2.api.vip_address
    node_index = count.index
  })

  scheduler_hints {
    group = openstack_compute_servergroup_v2.master.id
  }
}

resource "openstack_compute_servergroup_v2" "master" {
  name     = "social-protection-${var.environment}-master-group"
  policies = ["anti-affinity"]
}

resource "openstack_lb_member_v2" "master" {
  count         = var.master_count
  pool_id       = openstack_lb_pool_v2.api.id
  address       = openstack_compute_instance_v2.master[count.index].access_ip_v4
  protocol_port = 6443
  subnet_id     = openstack_networking_subnet_v2.main.id
}

# Worker Nodes
resource "openstack_compute_instance_v2" "worker" {
  count           = var.worker_count
  name            = "social-protection-${var.environment}-worker-${count.index}"
  image_id        = data.openstack_images_image_v2.ubuntu.id
  flavor_name     = "m1.2xlarge"
  key_pair        = var.key_pair_name
  security_groups = [openstack_networking_secgroup_v2.worker.name]

  network {
    uuid = openstack_networking_network_v2.main.id
  }

  user_data = file("${path.module}/templates/worker-init.sh")

  scheduler_hints {
    group = openstack_compute_servergroup_v2.worker.id
  }
}

resource "openstack_compute_servergroup_v2" "worker" {
  name     = "social-protection-${var.environment}-worker-group"
  policies = ["soft-anti-affinity"]
}

# Storage Nodes
resource "openstack_compute_instance_v2" "storage" {
  count           = var.storage_count
  name            = "social-protection-${var.environment}-storage-${count.index}"
  image_id        = data.openstack_images_image_v2.ubuntu.id
  flavor_name     = "m1.xlarge"
  key_pair        = var.key_pair_name
  security_groups = [openstack_networking_secgroup_v2.storage.name]

  network {
    uuid = openstack_networking_network_v2.main.id
  }

  user_data = file("${path.module}/templates/storage-init.sh")

  scheduler_hints {
    group = openstack_compute_servergroup_v2.storage.id
  }
}

resource "openstack_compute_servergroup_v2" "storage" {
  name     = "social-protection-${var.environment}-storage-group"
  policies = ["anti-affinity"]
}

resource "openstack_blockstorage_volume_v3" "storage_osd" {
  count       = var.storage_count
  name        = "social-protection-${var.environment}-osd-${count.index}"
  size        = 500
  volume_type = "ssd"
}

resource "openstack_compute_volume_attach_v2" "storage_osd" {
  count       = var.storage_count
  instance_id = openstack_compute_instance_v2.storage[count.index].id
  volume_id   = openstack_blockstorage_volume_v3.storage_osd[count.index].id
}

# Outputs
output "api_lb_floating_ip" {
  description = "Floating IP for Kubernetes API server"
  value       = openstack_networking_floatingip_v2.api.address
}

output "api_lb_vip" {
  description = "VIP address for Kubernetes API server"
  value       = openstack_lb_loadbalancer_v2.api.vip_address
}

output "master_ips" {
  description = "IP addresses of master nodes"
  value       = openstack_compute_instance_v2.master[*].access_ip_v4
}

output "worker_ips" {
  description = "IP addresses of worker nodes"
  value       = openstack_compute_instance_v2.worker[*].access_ip_v4
}

output "storage_ips" {
  description = "IP addresses of storage nodes"
  value       = openstack_compute_instance_v2.storage[*].access_ip_v4
}
