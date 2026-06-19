.PHONY: help dev-up dev-down tidy build push-all deploy-local clean

SERVICES := auth-service workout-service user-service menu-service export-service
ECR_REPO  ?= $(shell grep ECR_REPO .env 2>/dev/null | cut -d= -f2)

help: ## このヘルプを表示
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

# ===== ローカル開発 (docker-compose) =====

dev-up: ## docker-compose でローカル環境を起動
	docker compose up --build -d
	@echo "API Gateway: http://localhost:8080"
	@echo "Frontend:    http://localhost:3000 (direct)"
	@echo "Auth:        http://localhost:8081/health"
	@echo "Workout:     http://localhost:8082/health"
	@echo "User:        http://localhost:8083/health"
	@echo "Menu:        http://localhost:8084/health"
	@echo "Export:      http://localhost:8085/health"

dev-down: ## docker-compose を停止
	docker compose down

dev-logs: ## 全サービスのログを表示
	docker compose logs -f

# ===== Go ビルド =====

tidy: ## 全サービスの go mod tidy を実行
	@for svc in $(SERVICES); do \
		echo "→ tidy $$svc"; \
		cd services/$$svc && go mod tidy && cd ../..; \
	done

build: tidy ## 全サービスの Docker イメージをビルド
	@for svc in $(SERVICES); do \
		echo "→ build $$svc"; \
		docker build -t workoutmemo/$$svc:latest services/$$svc; \
	done

# ===== ECR プッシュ (AWS) =====

ecr-login: ## AWS ECR にログイン
	aws ecr get-login-password --region $(AWS_REGION) | \
		docker login --username AWS --password-stdin $(ECR_REPO)

push-all: ecr-login ## 全サービスを ECR にプッシュ
	@for svc in $(SERVICES); do \
		echo "→ push $$svc"; \
		docker build -t $(ECR_REPO)/$$svc:latest services/$$svc; \
		docker push $(ECR_REPO)/$$svc:latest; \
	done

# ===== kind ローカル K8s =====

kind-create: ## kind クラスタを作成
	kind create cluster --name workout-memo --config kind-config.yaml

kind-load: build ## ビルド済みイメージを kind にロード
	@for svc in $(SERVICES); do \
		kind load docker-image workoutmemo/$$svc:latest --name workout-memo; \
	done

deploy-local: kind-load ## kind クラスタへ全マニフェストをデプロイ
	kubectl apply -f k8s/namespaces.yaml
	kubectl apply -f k8s/postgres/
	kubectl apply -f k8s/redis/
	@echo "Waiting for postgres to be ready..."
	kubectl wait --for=condition=ready pod -l app=postgres -n workout-data --timeout=60s
	kubectl apply -f k8s/backend/
	kubectl apply -f k8s/auth/
	kubectl apply -f k8s/workout/
	kubectl apply -f k8s/user/
	kubectl apply -f k8s/menu/
	kubectl apply -f k8s/export/
	kubectl apply -f k8s/frontend/
	kubectl apply -f k8s/ingress/
	kubectl apply -f k8s/monitoring/
	@echo "デプロイ完了!"
	kubectl get pods -A

deploy-down: ## kind クラスタを削除
	kind delete cluster --name workout-memo

# ===== EKS デプロイ =====

eks-create: ## EKS クラスタを作成 (eksctl が必要)
	eksctl create cluster \
		--name $(EKS_CLUSTER_NAME) \
		--region $(AWS_REGION) \
		--node-type t3.medium \
		--nodes 2 \
		--nodes-min 2 \
		--nodes-max 4 \
		--with-oidc \
		--managed

eks-deploy: push-all ## EKS へデプロイ（push-all 後に実行）
	kubectl apply -f k8s/namespaces.yaml
	kubectl apply -f k8s/postgres/
	kubectl apply -f k8s/redis/
	kubectl apply -f k8s/backend/
	kubectl apply -f k8s/auth/
	kubectl apply -f k8s/workout/
	kubectl apply -f k8s/user/
	kubectl apply -f k8s/menu/
	kubectl apply -f k8s/export/
	kubectl apply -f k8s/frontend/
	kubectl apply -f k8s/ingress/
	kubectl apply -f k8s/monitoring/

# ===== ユーティリティ =====

status: ## 全 namespace の Pod 状態を確認
	kubectl get pods -A -o wide

clean: ## Docker イメージとボリュームを削除
	docker compose down -v
	docker rmi $$(docker images 'workoutmemo/*' -q) 2>/dev/null || true
