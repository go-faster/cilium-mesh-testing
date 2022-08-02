# Cilium clustermesh testing

This repo is a fork of [bmcustodio/kind-cilium-mesh](https://github.com/bmcustodio/kind-cilium-mesh).

## Introduction

This project bootstraps a [cluster mesh](https://docs.cilium.io/en/stable/gettingstarted/clustermesh/) between [kind](https://github.com/kubernetes-sigs/kind) clusters using [Cilium](https://cilium.io) which can be used for demo or testing purposes.

Each cluster in mesh has http echo server deployment that responds with pod and cluster name:
```json
{"cluster": "Cluster3", "pod": "deathstar-6578784ddd-m7hk5"}
```

And clients that send requests to deployment service, collecting information about requests distribution across clusters and pods based on the server response.

## Required tools

* go
* docker
* docker-compose
* kubectl
* kind
* cilium

## Bootstrapping clustermesh

Don't forget to check system limits:
```
fs.file-max=5000000
fs.inotify.max_user_watches = 524288
fs.inotify.max_user_instances = 512
```

To bootstrap the cluster mesh, run
```shell
make up
```

Or manually (useful for troubleshooting):
```shell
make create_cluster_1
make create_cluster_2
make create_cluster_3
make enable_mesh_cluster_1
make enable_mesh_cluster_2
make enable_mesh_cluster_3
make connect_clusters_3
make build_deathstar_image
make build_tiefighter_image
make deploy_cluster_1
make deploy_cluster_2
make deploy_cluster_3
```

Note that clusters are running in kubeproxy-free mode.

## Monitoring

Create prometheus and grafana containers:
```shell
make setup_monitoring
```
Grafana will be available at ```http://localhost:3000```