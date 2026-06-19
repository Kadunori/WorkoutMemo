# WorkoutMemo — 学習ドキュメント索引

このドキュメント群は `index.html` の筋トレアプリを  
**マイクロサービス × Docker × Kubernetes × AWS** で動かすための学習ガイドです。  
すべてのコード例はこのリポジトリの実際のファイルを参照しています。

---

## 読む順番

```
1. マイクロサービス → 2. Docker → 3. Kubernetes → 4. AWS
```

| # | ドキュメント | 内容 | 難易度 |
|---|------------|------|--------|
| 1 | [マイクロサービス](01_microservices.md) | モノリスとの比較、DB per Service、JWT フロー、Clean Architecture | ★☆☆ |
| 2 | [Docker](02_docker.md) | Dockerfile 解説、マルチステージビルド、docker-compose 全体起動 | ★☆☆ |
| 3 | [Kubernetes](03_kubernetes.md) | Pod/Deployment/Service/Ingress/StatefulSet/HPA/CronJob 全リソース | ★★☆ |
| 4 | [AWS / EKS](04_aws.md) | EKS/ECR/RDS/ALB/Secrets Manager、EKS デプロイ全手順、コスト | ★★★ |

---

## 学習ロードマップ

```
Phase 1: ローカル docker-compose で動かす
─────────────────────────────────────────
□ make tidy                   # go.sum 生成
□ make dev-up                 # 全サービス起動
□ curl でAPIを叩く             # 02_docker.md §8 参照
□ docker compose logs で学ぶ

Phase 2: kind でローカル K8s を体験
─────────────────────────────────────────
□ kind create cluster         # 03_kubernetes.md §13 参照
□ kubectl apply -f k8s/       # マニフェスト全適用
□ kubectl get pods -A         # Pod の状態確認
□ kubectl logs / exec         # デバッグ練習
□ kubectl rollout             # ローリングアップデート体験
□ HPA の動作確認               # 負荷をかけてスケールを見る

Phase 3: AWS EKS へ移行
─────────────────────────────────────────
□ ECR リポジトリ作成           # 04_aws.md §3 参照
□ make push-all               # イメージを ECR にプッシュ
□ make eks-create             # EKS クラスタ作成
□ RDS 作成・切り替え            # k8s/postgres/secret.yaml 更新
□ ALB Controller セットアップ
□ make eks-deploy
□ Route 53 + ACM で HTTPS 化
```

---

## ファイルマップ

```
WorkoutMemo/
├── index.html                ← 既存 PWA (変更なし)
│
├── services/                 ← Go マイクロサービス
│   ├── auth-service/         - JWT 登録・ログイン
│   ├── workout-service/      - セッション・セット CRUD + 前回重量
│   ├── user-service/         - プロフィール・体重記録
│   ├── menu-service/         - 種目マスタ (デフォルト種目シード済)
│   └── export-service/       - サービス間呼び出しで CSV 生成
│
├── k8s/                      ← Kubernetes マニフェスト
│   ├── namespaces.yaml         4 Namespace
│   ├── postgres/               StatefulSet + Headless Service + DB 初期化
│   ├── redis/                  StatefulSet
│   ├── auth/ workout/ user/ menu/ export/  Deployment + Service
│   ├── workout/hpa.yaml        HPA (CPU/MEM 自動スケール)
│   ├── frontend/               nginx Deployment
│   ├── ingress/                パスルーティング (kind/ALB 切替可)
│   ├── export/cronjob.yaml     週次バックアップ CronJob
│   └── monitoring/             Prometheus + Grafana
│
├── docs/                     ← このドキュメント群
├── docker-compose.yml        ← ローカル全サービス起動
├── nginx-dev.conf            ← 開発用リバースプロキシ
├── kind-config.yaml          ← kind クラスタ設定
├── Makefile                  ← make dev-up / kind / eks
└── .env.example              ← 環境変数テンプレート
```

---

## API エンドポイント早見表

| メソッド | パス | サービス | 説明 |
|---------|------|---------|------|
| POST | /auth/register | auth | ユーザー登録 |
| POST | /auth/login | auth | ログイン (JWT取得) |
| GET | /auth/validate | auth | トークン検証 |
| POST | /workouts | workout | セッション作成 |
| GET | /workouts | workout | セッション一覧 |
| GET | /workouts/:id | workout | セッション詳細 (sets含む) |
| POST | /workouts/:id/sets | workout | セット追加 |
| PUT | /workouts/:id/sets/:setId | workout | セット更新 |
| GET | /workouts/last-set?exercise=X&set=1 | workout | 前回重量取得 |
| GET | /users/profile | user | プロフィール取得 |
| PUT | /users/profile | user | プロフィール更新 |
| GET | /users/weight | user | 体重記録一覧 |
| POST | /users/weight | user | 体重記録追加 |
| GET | /menus/defaults | menu | デフォルト種目一覧 |
| GET | /menus | menu | ユーザー種目リスト |
| POST | /menus | menu | 種目追加 |
| PUT | /menus/order | menu | 並び替え |
| GET | /export/workouts | export | CSV ダウンロード |
