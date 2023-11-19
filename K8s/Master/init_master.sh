#!/bin/bash

# init master-node

sudo systemctl enable kubelet
# pull container images
sudo kubeadm config images pull
# bootstrap cluster without DNS
sudo sysctl -p
sudo kubeadm init \
  --apiserver-advertise-address 172.20.1.22
  --pod-network-cidr=172.24.0.0/16 \
  --cri-socket unix:///run/containerd/containerd.sock > token


