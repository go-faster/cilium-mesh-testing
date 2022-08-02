# export KUBECONFIG=.kubeconfig.yaml

KUBE_SYSTEM_NAMESPACE = kube-system
CILIUM_NAMESPACE      = ${KUBE_SYSTEM_NAMESPACE}
CLUSTER_1_NAME        = mesh-1
CLUSTER_2_NAME        = mesh-2
CLUSTER_3_NAME        = mesh-3
CLUSTER_1_CONTEXT     = kind-${CLUSTER_1_NAME}
CLUSTER_2_CONTEXT     = kind-${CLUSTER_2_NAME}
CLUSTER_3_CONTEXT     = kind-${CLUSTER_3_NAME}

create_cluster_1:
	kind delete clusters ${CLUSTER_1_NAME}
	kind create cluster --retain -v 1 --name ${CLUSTER_1_NAME} --config _k8s/${CLUSTER_1_NAME}/kind.yaml
	helm upgrade --kube-context ${CLUSTER_1_CONTEXT} --install --namespace kube-system --repo https://helm.cilium.io cilium cilium --values _k8s/${CLUSTER_1_NAME}/helm.yaml
	cilium status --context ${CLUSTER_1_CONTEXT} --wait

create_cluster_2:
	kind delete clusters ${CLUSTER_2_NAME}
	kind create cluster --retain -v 1 --name ${CLUSTER_2_NAME} --config _k8s/${CLUSTER_2_NAME}/kind.yaml
	helm upgrade --kube-context ${CLUSTER_2_CONTEXT} --install --namespace kube-system --repo https://helm.cilium.io cilium cilium --values _k8s/${CLUSTER_2_NAME}/helm.yaml
	cilium status --context ${CLUSTER_2_CONTEXT} --wait

create_cluster_3:
	kind delete clusters ${CLUSTER_3_NAME}
	kind create cluster --retain -v 1 --name ${CLUSTER_3_NAME} --config _k8s/${CLUSTER_3_NAME}/kind.yaml
	helm upgrade --kube-context ${CLUSTER_3_CONTEXT} --install --namespace kube-system --repo https://helm.cilium.io cilium cilium --values _k8s/${CLUSTER_3_NAME}/helm.yaml
	cilium status --context ${CLUSTER_3_CONTEXT} --wait

enable_mesh_cluster_1:
	cilium clustermesh enable --context ${CLUSTER_1_CONTEXT} --service-type NodePort
	cilium clustermesh status --context ${CLUSTER_1_CONTEXT} --wait

enable_mesh_cluster_2:
	cilium clustermesh enable --context ${CLUSTER_2_CONTEXT} --service-type NodePort
	cilium clustermesh status --context ${CLUSTER_2_CONTEXT} --wait

enable_mesh_cluster_3:
	cilium clustermesh enable --context ${CLUSTER_3_CONTEXT} --service-type NodePort
	cilium clustermesh status --context ${CLUSTER_3_CONTEXT} --wait

connect_clusters_2:
	cilium clustermesh connect --context ${CLUSTER_1_CONTEXT} --destination-context ${CLUSTER_2_CONTEXT}
	cilium clustermesh status --context ${CLUSTER_1_CONTEXT} --wait
	cilium clustermesh status --context ${CLUSTER_2_CONTEXT} --wait

connect_clusters_3:
	cilium clustermesh connect --context ${CLUSTER_1_CONTEXT} --destination-context ${CLUSTER_2_CONTEXT}
	cilium clustermesh connect --context ${CLUSTER_1_CONTEXT} --destination-context ${CLUSTER_3_CONTEXT}
	cilium clustermesh connect --context ${CLUSTER_2_CONTEXT} --destination-context ${CLUSTER_3_CONTEXT}
	cilium clustermesh status --context ${CLUSTER_1_CONTEXT} --wait
	cilium clustermesh status --context ${CLUSTER_2_CONTEXT} --wait
	cilium clustermesh status --context ${CLUSTER_3_CONTEXT} --wait

disable_mesh_cluster_1:
	cilium clustermesh disable --context ${CLUSTER_1_CONTEXT}

disable_mesh_cluster_2:
	cilium clustermesh disable --context ${CLUSTER_2_CONTEXT}

disable_mesh_cluster_3:
	cilium clustermesh disable --context ${CLUSTER_3_CONTEXT}

build_deathstar_image:
	docker build -t deathstar:latest -f Dockerfile.deathstar .

build_tiefighter_image:
	docker build -t tiefighter:latest -f Dockerfile.tiefighter .

deploy_cluster_1:
	kind load --name=${CLUSTER_1_NAME} docker-image deathstar:latest
	kind load --name=${CLUSTER_1_NAME} docker-image tiefighter:latest
	kubectl --context ${CLUSTER_1_CONTEXT} apply -f _k8s/cluster1.yaml
	kubectl --context ${CLUSTER_1_CONTEXT} wait -n default --for=condition=available deployment/deathstar

deploy_cluster_2:
	kind load --name=${CLUSTER_2_NAME} docker-image deathstar:latest
	kind load --name=${CLUSTER_2_NAME} docker-image tiefighter:latest
	kubectl --context ${CLUSTER_2_CONTEXT} apply -f _k8s/cluster2.yaml
	kubectl --context ${CLUSTER_2_CONTEXT} wait -n default --for=condition=available deployment/deathstar

deploy_cluster_3:
	kind load --name=${CLUSTER_3_NAME} docker-image deathstar:latest
	kind load --name=${CLUSTER_3_NAME} docker-image tiefighter:latest
	kubectl --context ${CLUSTER_3_CONTEXT} apply -f _k8s/cluster3.yaml
	kubectl --context ${CLUSTER_3_CONTEXT} wait -n default --for=condition=available deployment/deathstar

up: create_cluster_1 \
	create_cluster_2 \
	create_cluster_3 \
	enable_mesh_cluster_1 \
	enable_mesh_cluster_2 \
	enable_mesh_cluster_3 \
	connect_clusters_3 \
	build_deathstar_image \
	build_tiefighter_image \
	deploy_cluster_1 \
	deploy_cluster_2 \
	deploy_cluster_3 \

setup_monitoring:
	cd _monitoring && docker-compose up -d --wait
	./_hack/wait.sh http://localhost:9090/-/healthy
	./_hack/wait.sh http://localhost:3000/api/health

# http://localhost:30007 - Cluster 1
# http://localhost:30008 - Cluster 2
# http://localhost:30009 - Cluster 3
# run_tiefighter:
# 	go run ./cmd/tiefighter --target-addr="http://localhost:30007" --listen-addr=":1337"
