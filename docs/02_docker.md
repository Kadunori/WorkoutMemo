# Docker 入門 — WorkoutMemo で学ぶ

## 1. Docker とは

**問題**: 「自分の Mac では動くが、サーバーでは動かない」  
**解決**: アプリと動作環境をまるごと **コンテナ** に封じ込める

```
┌─────────────────────────────────────────┐
│  Mac (ホストOS)                          │
│  ┌──────────────┐  ┌──────────────────┐ │
│  │ コンテナA     │  │ コンテナB         │ │
│  │ auth-service │  │ PostgreSQL 16    │ │
│  │ Go 1.22      │  │ データ永続化      │ │
│  └──────────────┘  └──────────────────┘ │
│       Docker Engine (仮想化レイヤー)      │
└─────────────────────────────────────────┘
```

### 主要概念

| 用語 | 説明 | 例 |
|------|------|-----|
| **Image** | コンテナの設計図（読み取り専用） | `golang:1.22-alpine` |
| **Container** | Image から起動した実行インスタンス | 動いている auth-service |
| **Dockerfile** | Image のビルド手順書 | `services/auth-service/Dockerfile` |
| **Registry** | Image の保管場所 | Docker Hub, AWS ECR |
| **Volume** | コンテナ外へのデータ永続化 | PostgreSQL のデータ |

---

## 2. Dockerfile の解説

auth-service の Dockerfile を読み解く。

ファイル: [services/auth-service/Dockerfile](../services/auth-service/Dockerfile)

```dockerfile
# ===== Stage 1: ビルド =====
FROM golang:1.22-alpine AS builder   # Go コンパイラ入りの軽量イメージ
WORKDIR /app                          # 作業ディレクトリを設定

COPY go.mod go.sum ./                 # 依存関係ファイルを先にコピー
RUN go mod download                   # ← 依存をキャッシュ（Dockerfile最適化の要点）

COPY . .                              # ソースコードをコピー
RUN CGO_ENABLED=0 GOOS=linux \        # Linux向けに静的バイナリをビルド
    go build -trimpath \
    -o /bin/auth-service ./cmd

# ===== Stage 2: 実行イメージ =====
FROM alpine:3.20                      # 実行には Go コンパイラ不要 → 超軽量
RUN apk add --no-cache ca-certificates # HTTPS通信のためのCA証明書
COPY --from=builder /bin/auth-service /bin/auth-service  # バイナリだけコピー

EXPOSE 8080
USER nobody                           # セキュリティ: root で動かさない
ENTRYPOINT ["/bin/auth-service"]
```

### マルチステージビルドの効果

```
ビルダーステージ:  ~400MB (Go SDK 込み)
      ↓ バイナリだけ抽出
実行ステージ:       ~15MB (alpine + バイナリのみ)
```

ビルドイメージが大きいほど、セキュリティリスク・転送時間・ストレージコストが増える。

---

## 3. go.mod と go.sum について

```dockerfile
COPY go.mod go.sum ./    # ← ソースより先にコピーするのがポイント
RUN go mod download
COPY . .                 # その後ソースをコピー
```

**なぜこの順番?**  
Docker はレイヤーキャッシュを持つ。ソースのみが変更された場合、`go mod download` はキャッシュを再利用する → ビルドが速い。

```
初回ビルド:  go.mod コピー → download → ソースコピー → build  (全実行)
2回目以降:   go.mod 変化なし → [キャッシュ利用] → ソースコピー → build
```

> **手順**: 各サービスを初めてビルドする前に `make tidy` を実行して `go.sum` を生成する。

---

## 4. docker-compose で全サービスを起動

ファイル: [docker-compose.yml](../docker-compose.yml)

```yaml
services:
  postgres:          # データベース
  redis:             # キャッシュ
  migrate-auth:      # マイグレーション（起動時1回のみ）
  auth-service:      # 認証サービス  → port 8081
  workout-service:   # ワークアウト  → port 8082
  user-service:      # ユーザー      → port 8083
  menu-service:      # メニュー      → port 8084
  export-service:    # エクスポート  → port 8085
  frontend:          # nginx + HTML  → port 3000
  nginx-proxy:       # リバースプロキシ→ port 8080 (全APIの入口)
```

### サービス間の依存関係

```
postgres (healthy?)
    ↓
migrate-auth (completed?)
    ↓
auth-service (起動)
```

```yaml
auth-service:
  depends_on:
    migrate-auth:
      condition: service_completed_successfully  # マイグレ完了後に起動
```

### よく使うコマンド

```bash
# 起動（--build でイメージ再ビルド）
make dev-up
# または
docker compose up --build -d

# ログを見る
docker compose logs -f auth-service
docker compose logs -f workout-service

# 停止
make dev-down

# コンテナの中に入る（デバッグ用）
docker compose exec auth-service sh
docker compose exec postgres psql -U workout -d auth_db

# 状態確認
docker compose ps
docker compose top
```

---

## 5. ネットワーク通信

docker-compose 内のサービスは **サービス名** で互いを参照できる。

```yaml
export-service:
  environment:
    WORKOUT_SERVICE_URL: "http://workout-service:8080"  # ← サービス名で解決
    USER_SERVICE_URL: "http://user-service:8080"
```

```
nginx-proxy (port 8080)
    ├── /auth      → http://auth-service:8080
    ├── /workouts  → http://workout-service:8080
    ├── /users     → http://user-service:8080
    ├── /menus     → http://menu-service:8080
    └── /export    → http://export-service:8080
```

リバースプロキシ設定: [nginx-dev.conf](../nginx-dev.conf)

---

## 6. Volume によるデータ永続化

```yaml
postgres:
  volumes:
    - postgres_data:/var/lib/postgresql/data  # ← named volume

volumes:
  postgres_data:   # コンテナ停止後もデータが残る
  redis_data:
```

```bash
# volume 一覧
docker volume ls

# volume の中身を確認
docker volume inspect workoutmemo_postgres_data

# volume も含めて全削除（データ消去！）
docker compose down -v
```

---

## 7. 環境変数の管理

```bash
# .env.example をコピーして .env を作成
cp .env.example .env
```

[.env.example](../.env.example) の内容:

```env
JWT_SECRET=dev-jwt-secret-change-in-production
AUTH_DATABASE_URL=postgres://workout:devpassword@postgres:5432/auth_db?sslmode=disable
```

> ⚠️ `.env` ファイルは絶対に git に commit しない。`.gitignore` に追加する。

---

## 8. ローカル動作確認フロー

```bash
# 1. go.sum を生成
make tidy

# 2. 全サービス起動
make dev-up

# 3. ヘルスチェック
curl http://localhost:8081/health  # auth
curl http://localhost:8082/health  # workout
curl http://localhost:8083/health  # user
curl http://localhost:8084/health  # menu

# 4. ユーザー登録
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'

# 5. ログイン → token 取得
TOKEN=$(curl -s -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}' | jq -r .token)

# 6. ワークアウトセッション作成
curl -X POST http://localhost:8080/workouts \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"muscle_group":"chest"}'
```

---

## 9. Image のサイズ最適化ポイント

| 手法 | 効果 |
|------|------|
| マルチステージビルド | Go SDK 除外 → 400MB → 15MB |
| `alpine` ベース | 最小 OS |
| `CGO_ENABLED=0` | C ライブラリ不要 → 完全静的バイナリ |
| `-trimpath` | デバッグパス情報削除 → わずかに小さく |
| `--no-cache` (apk) | パッケージキャッシュ不要 |

```bash
# イメージサイズ確認
docker images | grep workoutmemo
```
