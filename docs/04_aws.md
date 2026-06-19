# AWS + EKS 入門 — WorkoutMemo で学ぶ

## 1. このプロジェクトで使う AWS サービス一覧

```
┌─────────────────────────────────────────────────────────────┐
│  インターネット                                               │
│        ↓                                                    │
│  Route 53 (DNS)  →  ACM (HTTPS証明書)                       │
│        ↓                                                    │
│  ALB (Application Load Balancer)                            │
│    ← AWS Load Balancer Controller (K8s Ingress)             │
│        ↓                                                    │
│  EKS (Elastic Kubernetes Service)                            │
│    ├── EC2 Worker Nodes (t3.medium × 2〜4)                  │
│    ├── ECR からイメージ Pull                                  │
│    ├── Secrets Manager → K8s Secret                         │
│    └── サービス → RDS PostgreSQL                             │
│                                                             │
│  ECR (コンテナレジストリ)                                    │
│  RDS PostgreSQL (マネージドDB)                               │
│  AWS Secrets Manager (機密情報)                              │
│  CloudWatch (ログ・監視)                                     │
└─────────────────────────────────────────────────────────────┘
```

---

## 2. 各 AWS サービスの役割

### EKS (Elastic Kubernetes Service)
Kubernetes のコントロールプレーンを AWS がマネージドで提供する。

```
自前 K8s:  マスターノード管理 + etcd バックアップ + アップグレード = 運用負荷大
EKS:       コントロールプレーンは AWS が管理 → ワーカーノードの管理だけでよい
```

```bash
# eksctl でクラスタ作成 (Makefile の eks-create ターゲット)
eksctl create cluster \
  --name workout-memo-cluster \
  --region ap-northeast-1 \         # 東京リージョン
  --node-type t3.medium \           # ワーカーノードのインスタンスタイプ
  --nodes 2 \                       # 通常時のノード数
  --nodes-min 2 \                   # 最小
  --nodes-max 4 \                   # 最大 (Cluster Autoscaler 用)
  --with-oidc \                     # IAM Roles for Service Accounts (IRSA) 用
  --managed                         # マネージドノードグループ (推奨)
```

### ECR (Elastic Container Registry)
Docker Hub の AWS 版。プライベートなコンテナレジストリ。

```bash
# ログイン
aws ecr get-login-password --region ap-northeast-1 | \
  docker login --username AWS --password-stdin \
  <account_id>.dkr.ecr.ap-northeast-1.amazonaws.com

# リポジトリ作成 (各サービス用)
aws ecr create-repository --repository-name workoutmemo/auth-service --region ap-northeast-1
aws ecr create-repository --repository-name workoutmemo/workout-service --region ap-northeast-1
# ...

# イメージ push (Makefile の push-all ターゲット)
docker build -t <ECR_REPO>/auth-service:latest services/auth-service
docker push <ECR_REPO>/auth-service:latest
```

### RDS (Relational Database Service) — PostgreSQL

ローカルでは K8s StatefulSet を使うが、EKS 本番では RDS に切り替える。

```
ローカル: PostgreSQL StatefulSet (k8s/postgres/)
    ↓ EKS 移行時に切り替え
EKS:     RDS PostgreSQL (マルチ AZ, 自動バックアップ, フェイルオーバー)
```

**切り替え方**: Deployment の `DATABASE_URL` 環境変数を変更するだけ

```yaml
# k8s/postgres/secret.yaml を更新
AUTH_DATABASE_URL: "postgres://workout:pass@<rds-endpoint>:5432/auth_db?sslmode=require"
```

RDS 作成例:
```bash
aws rds create-db-instance \
  --db-instance-identifier workout-memo-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --engine-version "16.3" \
  --master-username workout \
  --master-user-password <password> \
  --allocated-storage 20 \
  --vpc-security-group-ids <sg-id> \
  --db-subnet-group-name <subnet-group> \
  --multi-az \
  --region ap-northeast-1
```

### ACM (AWS Certificate Manager)
HTTPS 証明書を無料で発行・自動更新してくれる。

```bash
# 証明書リクエスト
aws acm request-certificate \
  --domain-name workout.example.com \
  --validation-method DNS \
  --region ap-northeast-1
```

### AWS Secrets Manager
K8s Secret の本番版。ローテーション・監査ログに対応。

```
AWS Secrets Manager
    ↓ External Secrets Operator (OSSツール)
K8s ExternalSecret リソース
    ↓ 自動同期
K8s Secret
    ↓ secretKeyRef
Pod 環境変数
```

```yaml
# ExternalSecret の例
apiVersion: external-secrets.io/v1beta1
kind: ExternalSecret
metadata:
  name: app-secret
  namespace: workout-backend
spec:
  secretStoreRef:
    name: aws-secretsmanager
  target:
    name: app-secret
  data:
    - secretKey: JWT_SECRET
      remoteRef:
        key: workout-memo/jwt-secret
```

### ALB (Application Load Balancer) + AWS Load Balancer Controller

K8s の Ingress リソースを ALB に変換する。

ファイル: [k8s/ingress/ingress.yaml](../k8s/ingress/ingress.yaml) のアノテーション切替

```yaml
# ローカル kind 用
annotations:
  kubernetes.io/ingress.class: nginx

# AWS EKS 用 (コメントアウトを切り替え)
annotations:
  kubernetes.io/ingress.class: alb
  alb.ingress.kubernetes.io/scheme: internet-facing
  alb.ingress.kubernetes.io/target-type: ip
  alb.ingress.kubernetes.io/certificate-arn: <ACM_CERT_ARN>
```

ALB Controller インストール:
```bash
# Helm でインストール
helm repo add eks https://aws.github.io/eks-charts
helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=workout-memo-cluster \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller
```

### Route 53
ドメインの DNS 管理。ALB に独自ドメインを向ける。

```bash
# ALB のエンドポイントを確認
kubectl get ingress -n workout-frontend

# Route 53 に A レコード (エイリアス) を追加
# workout.example.com → ALB DNS 名
```

### CloudWatch
EKS のログ・メトリクスを収集する。

```bash
# Container Insights 有効化
eksctl utils associate-iam-oidc-provider --cluster workout-memo-cluster --approve
aws eks update-addon --cluster-name workout-memo-cluster \
  --addon-name amazon-cloudwatch-observability \
  --region ap-northeast-1
```

---

## 3. EKS デプロイ全手順

### 前提ツール

```bash
# インストール確認
aws --version          # AWS CLI v2
eksctl version         # eksctl
kubectl version        # kubectl
helm version           # Helm 3
docker --version       # Docker
```

### Step 1: クラスタ作成

```bash
cp .env.example .env
# .env を編集: AWS_REGION, EKS_CLUSTER_NAME, ECR_REPO を設定

make eks-create
# または
eksctl create cluster --name workout-memo --region ap-northeast-1 \
  --node-type t3.medium --nodes 2 --nodes-min 2 --nodes-max 4 \
  --with-oidc --managed
```

### Step 2: ECR リポジトリ作成

```bash
for svc in auth-service workout-service user-service menu-service export-service; do
  aws ecr create-repository \
    --repository-name workoutmemo/$svc \
    --region ap-northeast-1
done
```

### Step 3: イメージビルド & プッシュ

```bash
make tidy      # go.sum 生成
make push-all  # ビルド + ECR プッシュ
```

### Step 4: ALB Controller + OIDC セットアップ

```bash
# OIDC プロバイダーを EKS に紐付け
eksctl utils associate-iam-oidc-provider \
  --cluster workout-memo --approve --region ap-northeast-1

# ALB Controller 用 IAM ポリシー
curl -o alb-policy.json \
  https://raw.githubusercontent.com/kubernetes-sigs/aws-load-balancer-controller/main/docs/install/iam_policy.json

aws iam create-policy \
  --policy-name AWSLoadBalancerControllerIAMPolicy \
  --policy-document file://alb-policy.json

eksctl create iamserviceaccount \
  --cluster workout-memo \
  --namespace kube-system \
  --name aws-load-balancer-controller \
  --attach-policy-arn arn:aws:iam::<ACCOUNT_ID>:policy/AWSLoadBalancerControllerIAMPolicy \
  --approve

helm install aws-load-balancer-controller eks/aws-load-balancer-controller \
  -n kube-system \
  --set clusterName=workout-memo \
  --set serviceAccount.create=false \
  --set serviceAccount.name=aws-load-balancer-controller
```

### Step 5: RDS 作成（オプション）

```bash
# VPC/Subnet グループは EKS と同じ VPC を使う
aws rds create-db-instance \
  --db-instance-identifier workout-memo-db \
  --db-instance-class db.t3.micro \
  --engine postgres \
  --engine-version "16.3" \
  --master-username workout \
  --master-user-password <strong-password> \
  --allocated-storage 20 \
  --multi-az \
  --region ap-northeast-1
```

### Step 6: Secret を更新して EKS にデプロイ

```bash
# k8s/postgres/secret.yaml の DATABASE_URL を RDS エンドポイントに更新
# k8s/backend/secret.yaml の JWT_SECRET を本番用に更新 (openssl rand -hex 32)
# k8s/ingress/ingress.yaml のアノテーションを ALB 用に切り替え

make eks-deploy
```

### Step 7: 動作確認

```bash
# ALB エンドポイント確認
kubectl get ingress -n workout-frontend

# Pod が全て Running になっているか確認
kubectl get pods -A

# ログ確認
kubectl logs -f deploy/auth-service -n workout-backend
```

---

## 4. コスト見積もり（東京リージョン）

| サービス | スペック | 月額概算 |
|----------|---------|---------|
| EKS クラスタ | コントロールプレーン | ~$73 |
| EC2 (t3.medium × 2) | ワーカーノード | ~$60 |
| RDS (db.t3.micro) | PostgreSQL シングル AZ | ~$15 |
| ALB | ロードバランサー | ~$20 |
| ECR | 5サービス × ~50MB | ~$1 |
| Route 53 | ホストゾーン + クエリ | ~$1 |
| **合計** | | **~$170/月** |

> 学習用なら: `eksctl delete cluster` でこまめに削除するか、Fargate 利用でノード代を削減できる。

**コスト削減のヒント**:
- 使わない時間帯はワーカーノードを 0 に (`eksctl scale nodegroup --nodes 0`)
- `t3.micro` (フリーティア) を使う (リソース制限に注意)
- Fargate Profile を使うとノード管理不要 (Pod 単位の課金)

---

## 5. ローカル kind → EKS 移行チェックリスト

```
□ .env の ECR_REPO, AWS_REGION, EKS_CLUSTER_NAME を設定
□ k8s/postgres/secret.yaml の DATABASE_URL を RDS エンドポイントに更新
□ k8s/backend/secret.yaml の JWT_SECRET を openssl rand -hex 32 で生成
□ k8s/ingress/ingress.yaml のアノテーションを ALB 用に変更
□ k8s/ingress/ingress.yaml の host を本番ドメインに変更
□ make push-all でイメージを ECR にプッシュ
□ k8s/auth/deployment.yaml の image: を ECR の URI に更新 (全サービス)
□ make eks-deploy
□ Route 53 で ALB エンドポイントに CNAME/エイリアスを追加
□ ACM 証明書の DNS 検証が通っているか確認
□ kubectl get pods -A で全 Pod が Running か確認
```

---

## 6. IAM の基本 — 最小権限の原則

```
❌ 悪い例: ルートアカウントでEKS操作
✅ 良い例: 専用 IAM ユーザー / ロールを作成

# 開発者ユーザーに必要な権限
- eks:*          (EKS 操作)
- ecr:*          (ECR 操作)
- ec2:Describe*  (VPC/SG 確認)

# EKS Worker Node ロールに必要な権限
- AmazonEKSWorkerNodePolicy
- AmazonEKS_CNI_Policy
- AmazonEC2ContainerRegistryReadOnly
```

eksctl はこれらを自動で設定してくれる。
