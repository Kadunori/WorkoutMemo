# Kubernetes 入門 — WorkoutMemo で学ぶ

## 1. Kubernetes とは

**問題**: docker-compose は1台のサーバーでしか動かない  
**解決**: 複数サーバー（Node）に跨ってコンテナを自動配置・管理する

```
K8s クラスタ
  ┌─────────────────────────────────────────────────────┐
  │  Control Plane (マスター)                            │
  │    API Server ← kubectl コマンドの受け口             │
  │    Scheduler  ← Pod をどの Node に置くか決める       │
  │    etcd       ← クラスタの状態を保存する KV ストア   │
  │                                                     │
  │  Worker Node 1          Worker Node 2               │
  │  ┌─────────────────┐   ┌─────────────────┐         │
  │  │ auth-service Pod│   │ auth-service Pod│         │
  │  │ workout-service │   │ workout-service │         │
  │  └─────────────────┘   └─────────────────┘         │
  └─────────────────────────────────────────────────────┘
```

---

## 2. 主要リソース一覧

| リソース | 役割 | このプロジェクトでの使用例 |
|----------|------|--------------------------|
| **Pod** | コンテナの最小実行単位 | auth-service の1インスタンス |
| **Deployment** | Pod の宣言的管理・ローリングアップデート | 全バックエンドサービス |
| **Service** | Pod への安定したネットワークアクセス | auth-service → port 8080 |
| **Ingress** | 外部からのHTTPルーティング | /auth → auth-service |
| **StatefulSet** | 順序付き・固定IDの Pod 管理 | PostgreSQL, Redis |
| **HPA** | CPU/Memory に応じた自動スケール | workout-service |
| **CronJob** | 定期実行ジョブ | 週次CSVバックアップ |
| **ConfigMap** | 設定値（非機密情報）の管理 | nginx 設定, DB init SQL |
| **Secret** | 機密情報の管理 | JWT_SECRET, DB パスワード |
| **Namespace** | リソースの論理グループ分け | frontend / backend / data / monitoring |
| **ServiceAccount** | Pod に紐づく権限 | Prometheus の K8s API アクセス |
| **ClusterRole** | クラスタ全体の権限定義 | Prometheus の Pod 一覧取得 |

---

## 3. Namespace 設計

ファイル: [k8s/namespaces.yaml](../k8s/namespaces.yaml)

```yaml
workout-frontend   ← nginx + Ingress
workout-backend    ← 5つのGoサービス + Secret
workout-data       ← PostgreSQL + Redis (StatefulSet)
workout-monitoring ← Prometheus + Grafana
```

**なぜ分けるか?**
- アクセス制御（NetworkPolicy）を Namespace 単位で適用できる
- リソースクォータを設定できる（data namespace は Pod 数を制限など）
- `kubectl get pods -n workout-backend` で絞り込みやすい

---

## 4. Deployment — サービスの宣言的管理

ファイル: [k8s/workout/deployment.yaml](../k8s/workout/deployment.yaml)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: workout-service
  namespace: workout-backend
spec:
  replicas: 2              # ← 「常に2つの Pod を維持せよ」という宣言
  selector:
    matchLabels:
      app: workout-service # ← このラベルの Pod を管理する
  template:
    metadata:
      labels:
        app: workout-service
    spec:
      containers:
        - name: workout-service
          image: <ECR_REPO>/workout-service:latest
          env:
            - name: JWT_SECRET
              valueFrom:
                secretKeyRef:         # ← Secret から値を取得
                  name: app-secret
                  key: JWT_SECRET
          livenessProbe:              # ← 死活監視: 失敗したら Pod を再起動
            httpGet:
              path: /health
              port: 8080
          readinessProbe:             # ← 準備確認: OK になるまでトラフィックを送らない
            httpGet:
              path: /health
              port: 8080
          resources:
            requests:                 # ← スケジューリング時に必要な最低リソース
              memory: "32Mi"
              cpu: "50m"              # 50m = 0.05 CPU コア
            limits:                   # ← これを超えたら強制終了/throttle
              memory: "128Mi"
              cpu: "300m"
```

### ローリングアップデート

新しいイメージを push してもダウンタイムなしで更新できる。

```bash
# イメージを更新
kubectl set image deployment/workout-service \
  workout-service=<ECR_REPO>/workout-service:v2 \
  -n workout-backend

# 更新状況を確認
kubectl rollout status deployment/workout-service -n workout-backend

# 問題が起きたらロールバック
kubectl rollout undo deployment/workout-service -n workout-backend
```

---

## 5. Service — Pod への安定アクセス

Pod の IP は起動するたびに変わる。Service が安定した仮想 IP を提供する。

ファイル: [k8s/auth/service.yaml](../k8s/auth/service.yaml)

```yaml
apiVersion: v1
kind: Service
metadata:
  name: auth-service
  namespace: workout-backend
spec:
  selector:
    app: auth-service    # このラベルの Pod 全てにロードバランス
  ports:
    - port: 8080         # Service の受け口
      targetPort: 8080   # Pod の受け口
```

### Service の DNS 解決

```
# 同じ namespace から
http://auth-service:8080

# 異なる namespace から (FQDN)
http://auth-service.workout-backend.svc.cluster.local:8080
```

export-service から workout-service を呼ぶ例: [k8s/export/deployment.yaml](../k8s/export/deployment.yaml)

```yaml
- name: WORKOUT_SERVICE_URL
  value: "http://workout-service.workout-backend.svc.cluster.local:8080"
```

### ExternalName Service (namespace 跨ぎルーティング)

ファイル: [k8s/ingress/ingress.yaml](../k8s/ingress/ingress.yaml)

```yaml
# frontend namespace から backend namespace のサービスへ転送する
apiVersion: v1
kind: Service
metadata:
  name: auth-service-ext
  namespace: workout-frontend      # ← frontend namespace にある
spec:
  type: ExternalName
  externalName: auth-service.workout-backend.svc.cluster.local  # ← backend を指す
```

---

## 6. Ingress — 外部からのHTTPルーティング

ファイル: [k8s/ingress/ingress.yaml](../k8s/ingress/ingress.yaml)

```yaml
spec:
  rules:
    - host: workout.local
      http:
        paths:
          - path: /auth       → auth-service-ext:8080
          - path: /workouts   → workout-service-ext:8080
          - path: /users      → user-service-ext:8080
          - path: /menus      → menu-service-ext:8080
          - path: /export     → export-service-ext:8080
          - path: /           → frontend:80          # フロントエンド (最後)
```

```
インターネット
    ↓
Ingress (nginx / AWS ALB)
    ├── /auth/*      → auth-service     Pod
    ├── /workouts/*  → workout-service  Pod × 2
    ├── /users/*     → user-service     Pod × 2
    └── /*           → frontend         Pod × 2
```

---

## 7. StatefulSet — データベースの管理

通常の Deployment と何が違うか?

| | Deployment | StatefulSet |
|--|------------|-------------|
| Pod 名 | ランダム (postgres-abc12) | 固定 (postgres-0, postgres-1) |
| 起動順序 | 並列 | 0番から順番に |
| ストレージ | 共有 or なし | 各 Pod に専用 PVC |
| 用途 | ステートレスサービス | DB, Kafka, Redis |

ファイル: [k8s/postgres/statefulset.yaml](../k8s/postgres/statefulset.yaml)

```yaml
kind: StatefulSet
spec:
  serviceName: postgres      # Headless Service と紐付け
  replicas: 1
  volumeClaimTemplates:      # ← 各 Pod に専用 PVC を自動作成
    - metadata:
        name: data
      spec:
        accessModes: ["ReadWriteOnce"]
        resources:
          requests:
            storage: 5Gi
```

**Headless Service** (`clusterIP: None`):  
`postgres-0.postgres.workout-data.svc.cluster.local` のように Pod を直接 DNS で指定できる。

---

## 8. HPA — 自動スケール

ファイル: [k8s/workout/hpa.yaml](../k8s/workout/hpa.yaml)

```yaml
kind: HorizontalPodAutoscaler
spec:
  scaleTargetRef:
    name: workout-service
  minReplicas: 2    # 最低 2 Pod
  maxReplicas: 6    # 最大 6 Pod
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70   # CPU 70% 超えたら Pod 追加
```

```
平常時: [Pod1] [Pod2]
負荷増: [Pod1] [Pod2] [Pod3] [Pod4]  ← 自動追加
負荷減: [Pod1] [Pod2]                ← 自動削減
```

> 前提: Metrics Server が必要 (`kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml`)

---

## 9. CronJob — 定期実行

ファイル: [k8s/export/cronjob.yaml](../k8s/export/cronjob.yaml)

```yaml
kind: CronJob
spec:
  schedule: "0 15 * * 6"   # 毎週土曜 15:00 UTC = 日曜 0:00 JST
  concurrencyPolicy: Forbid # 前のジョブが終わるまで新しいのを起動しない
  jobTemplate:
    spec:
      template:
        spec:
          restartPolicy: OnFailure
```

cron 記法: `分 時 日 月 曜`

```
"0 15 * * 6"
 │  │  │ │ └── 曜日 (6=土曜)
 │  │  │ └──── 月 (* = 毎月)
 │  │  └────── 日 (* = 毎日)
 │  └───────── 時 (15 UTC)
 └──────────── 分 (0)
```

---

## 10. Secret — 機密情報の管理

ファイル: [k8s/backend/secret.yaml](../k8s/backend/secret.yaml)

```yaml
kind: Secret
type: Opaque
stringData:             # ← base64 自動変換
  JWT_SECRET: "changeme_generate_with_openssl_rand_hex_32"
```

```bash
# JWT_SECRET の生成
openssl rand -hex 32
```

**本番での管理方法** (EKS):

```
AWS Secrets Manager
    ↓ External Secrets Operator
K8s Secret (自動同期)
    ↓ secretKeyRef
Pod の環境変数
```

---

## 11. Prometheus + Grafana — 監視

ファイル: [k8s/monitoring/prometheus.yaml](../k8s/monitoring/prometheus.yaml)

```yaml
# 監視対象を列挙
scrape_configs:
  - job_name: 'workout-service'
    static_configs:
      - targets: ['workout-service.workout-backend.svc.cluster.local:8080']
```

Go サービスに Prometheus メトリクスを追加するには:

```go
import "github.com/gin-contrib/prom"

r := gin.Default()
p := ginprometheus.NewPrometheus("gin")
p.Use(r)
```

```bash
# Grafana へのアクセス (NodePort 経由)
kubectl port-forward svc/grafana 3001:3000 -n workout-monitoring
# ブラウザ: http://localhost:3001 (admin/admin)
```

---

## 12. kubectl コマンド早見表

```bash
# === 状態確認 ===
kubectl get pods -A                           # 全 namespace の Pod
kubectl get pods -n workout-backend -o wide   # namespace 指定・詳細表示
kubectl get all -n workout-backend            # Deployment/Service/Pod まとめて

# === デバッグ ===
kubectl logs -f deploy/workout-service -n workout-backend  # ログ追跡
kubectl describe pod <pod-name> -n workout-backend          # イベント確認
kubectl exec -it <pod-name> -n workout-backend -- sh        # コンテナに入る

# === デプロイ ===
kubectl apply -f k8s/workout/                # ディレクトリ内の全 yaml を適用
kubectl delete -f k8s/workout/              # 削除
kubectl rollout restart deploy/workout-service -n workout-backend  # 再起動

# === スケール ===
kubectl scale deploy/workout-service --replicas=4 -n workout-backend

# === ポートフォワード (ローカルで直接確認) ===
kubectl port-forward svc/auth-service 8081:8080 -n workout-backend
kubectl port-forward svc/postgres 5432:5432 -n workout-data

# === Secret の確認 ===
kubectl get secret app-secret -n workout-backend -o jsonpath='{.data.JWT_SECRET}' | base64 -d
```

---

## 13. ローカル kind での起動手順

```bash
# 1. kind クラスタ作成
make kind-create
# または: kind create cluster --name workout-memo --config kind-config.yaml

# 2. nginx Ingress Controller インストール
kubectl apply -f https://raw.githubusercontent.com/kubernetes/ingress-nginx/main/deploy/static/provider/kind/deploy.yaml
kubectl wait --namespace ingress-nginx \
  --for=condition=ready pod \
  --selector=app.kubernetes.io/component=controller \
  --timeout=90s

# 3. Docker イメージをビルドして kind にロード
make kind-load

# 4. マニフェストを全適用
make deploy-local

# 5. /etc/hosts に追加 (Mac)
echo "127.0.0.1 workout.local" | sudo tee -a /etc/hosts

# 6. ブラウザで確認
open http://workout.local:8080
```

---

## 14. マニフェストの適用順序

依存関係があるため、以下の順序で `kubectl apply` する。

```
1. namespaces.yaml         ← 最初に namespace を作る
2. postgres/               ← DB は先に起動
3. redis/
4. backend/secret.yaml     ← サービスより先に Secret
5. auth/ workout/ user/ menu/ export/  ← バックエンド
6. frontend/               ← フロントエンド
7. ingress/                ← 最後に外部公開
8. monitoring/             ← 任意
```

`make deploy-local` がこの順序を担当している: [Makefile](../Makefile)
