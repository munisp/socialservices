#!/bin/bash
set -ex

# Storage node initialization script for Ceph

# Update system
apt-get update && apt-get upgrade -y

# Install required packages
apt-get install -y apt-transport-https ca-certificates curl gnupg lsb-release python3-pip lvm2

# Install cephadm
curl --silent --remote-name --location https://github.com/ceph/ceph/raw/quincy/src/cephadm/cephadm
chmod +x cephadm
./cephadm add-repo --release quincy
./cephadm install

# Install ceph-common for CLI tools
apt-get install -y ceph-common

# Prepare OSD disk (assuming /dev/vdb is the OSD disk)
if [ -b /dev/vdb ]; then
  # Wipe the disk
  wipefs -a /dev/vdb
  
  # Create LVM physical volume
  pvcreate /dev/vdb
  
  # Create volume group for Ceph
  vgcreate ceph-vg /dev/vdb
  
  # Create logical volume for OSD
  lvcreate -l 100%FREE -n ceph-lv ceph-vg
fi

# Configure system for Ceph
cat <<EOF | tee /etc/sysctl.d/ceph.conf
# Ceph OSD tuning
vm.swappiness = 10
vm.min_free_kbytes = 1048576
kernel.pid_max = 4194304
EOF
sysctl --system

# Increase file descriptor limits
cat <<EOF | tee /etc/security/limits.d/ceph.conf
*               soft    nofile          1048576
*               hard    nofile          1048576
root            soft    nofile          1048576
root            hard    nofile          1048576
EOF

# Signal completion
echo "Storage node setup complete - ready for Ceph bootstrap" > /tmp/setup_complete
