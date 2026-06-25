# Cloudflare アーキテクチャ — WorkoutMemo

## 全体構成図

```
ユーザー (スマートフォン)
        │
        ▼
┌─────────────────────────────────────────────────────────────┐
│  Cloudflare Network (200+ エッジロケーション / 世界中)        │
│                                                             │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Cloudflare Pages                                    │  │
│  │  index.html (PWA) を全世界のエッジから配信             │  │
│  └──────────────────────────────────────────────────────┘  │
│           │ fetch('/api/auth/*')                           │
│           │ fetch('/api/workouts/*')                       │
│           ▼                                                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Cloudflare Workers (サーバーレス エッジ関数)          │  │
│  │                                                      │  │
│  │  workout-auth    ─ POST /api/auth/register|login     │  │
│  │  workout-workout ─ /api/workouts/*                   │  │
│  │  workout-user    ─ /api/users/*                      │  │
│  │  workout-menu    ─ /api/menus/*                      │  │
│  │  workout-export  ─ /api/export/*                     │  │
│  │       │  Service Bindings (同一DC・ゼロレイテンシ)    │  │
│  │       └──→ workout-workout, workout-user             │  │
│  └──────────────────────────────────────────────────────┘  │
│           │                                                │
│           ▼                                                │
│  ┌──────────────────────────────────────────────────────┐  │
│  │  Cloudflare D1 (SQLite・エッジDB)                    │  │
│  │  - users / workout_sessions / workout_sets           │  │
│  │  - user_profiles / body_weight_records               │  │
│  │  - exercises / user_exercises                        │  │
│  └──────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌──────────────────────┐  ┌──────────────────────────┐   │
│  │  Cloudflare KV       │  │  Cloudflare R2           │   │
│  │  (将来: セッション    │  │  CSV エクスポートの保存   │   │
│  │   キャッシュ等)       │  │  (S3 互換)               │   │
│  └──────────────────────┘  └──────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

---

## EKS vs Cloudflare 比較

| 項目 | EKS (k8s/) | Cloudflare (cloudflare/) |
|------|-----------|--------------------------|
| **バックエンド** | Go + gin | TypeScript + Hono |
| **実行環境** | K8s Pod (コンテナ) | Workers (サーバーレス V8) |
| **DB** | PostgreSQL (RDS) | D1 (SQLite at Edge) |
| **キャッシュ** | Redis StatefulSet | KV Store |
| **ファイル保存** | S3 | R2 |
| **CDN** | CloudFront (別途設定) | Cloudflare Network (込み) |
| **スケール** | HPA で Pod を追加 | 自動 (Cloudflare が管理) |
| **デプロイ** | `kubectl apply` | `wrangler deploy` (数秒) |
| **月額コスト** | ~$170 | **$0〜$5** (無料枠内) |
| **コールドスタート** | なし (常時起動) | < 5ms (V8 Isolate) |
| **学習コスト** | 高 (K8s 知識必要) | 低 |
| **適したケース** | 複雑な処理・長時間実行 | 低レイテンシ・グローバル配信 |

---

## 使用する Cloudflare サービスの解説

### Cloudflare Pages
静的ファイルを全世界のエッジから配信するホスティングサービス。

```
GitHub リポジトリ
    ↓ push → 自動ビルド&デプロイ
Cloudflare Pages
    → index.html を 200+ PoP から配信
    → HTTPS 自動、カスタムドメイン対応
```

### Cloudflare Workers
JavaScript/TypeScript をエッジで実行するサーバーレス関数。

```
EKS との違い:
  EKS:     コンテナ起動 (コールドスタート 数秒)
  Workers: V8 Isolate 起動 (< 5ms・事実上コールドスタートなし)

制限 (Free プラン):
  - CPU: 10ms/リクエスト
  - メモリ: 128MB
  - リクエスト: 100,000/日
  - Worker サイズ: 1MB (圧縮後)
```

### Cloudflare D1
Workers 専用の SQLite データベース。

```
PostgreSQL との違い:
  PostgreSQL: 独立したプロセス、接続プール必要
  D1:         Workers と同じエッジで動作、接続不要

SQLite の制限:
  - 書き込みは 1 Worker に制限 (読み込みはレプリカ可)
  - JOIN は使えるが複雑なクエリはパフォーマンスに注意
  - DB サイズ: 10GB まで (Free: 5GB)
```

### Service Bindings
Workers 間の通信。HTTP より高速（同一データセンター内、ネットワーク経路なし）。

```typescript
// export Worker が workout Worker を呼ぶ
const res = await c.env.WORKOUT_SERVICE.fetch('https://internal/api/workouts', {
  headers: { Authorization: `Bearer ${token}` },
});
```

### Cloudflare R2
S3 互換のオブジェクトストレージ。CSV エクスポートの保存に使用。

```
S3 との違い:
  - エグレス料金なし (S3 はデータ転送で課金)
  - R2 は無料枠: 10GB ストレージ / 100万リクエスト/月
```

---

## ファイル構成

```
cloudflare/
├── architecture.md             ← このファイル
├── Makefile                    ← wrangler コマンド集
├── d1/
│   ├── schema.sql              ← D1 テーブル定義 (SQLite)
│   └── seed_exercises.sql      ← デフォルト種目データ
└── workers/
    ├── shared/                 ← 全 Worker 共通ユーティリティ
    │   ├── jwt.ts              ← Web Crypto API ベース JWT
    │   ├── password.ts         ← PBKDF2 パスワードハッシュ
    │   └── auth-middleware.ts  ← Hono JWT ミドルウェア
    ├── auth/                   ← 認証 Worker
    │   ├── src/index.ts
    │   ├── wrangler.toml
    │   └── package.json
    ├── workout/                ← ワークアウト Worker
    ├── user/                   ← ユーザー Worker
    ├── menu/                   ← メニュー Worker
    └── export/                 ← エクスポート Worker (Service Bindings 使用)
```

---

## デプロイ手順

### 前提条件
```bash
# Cloudflare アカウント作成 (無料)
# https://dash.cloudflare.com

# wrangler インストール
npm install -g wrangler

# ログイン
wrangler login
```

### Step 1: D1 データベース作成

```bash
cd cloudflare
make db-create
# 表示された database_id を全 wrangler.toml の database_id に記入
```

### Step 2: スキーマ適用 & シードデータ投入

```bash
make db-migrate   # テーブル作成
make db-seed      # デフォルト種目データ投入
```

### Step 3: JWT シークレット設定

```bash
make secret
# openssl rand -hex 32 で生成し、全 Worker に wrangler secret put で設定
```

### Step 4: wrangler.toml のドメイン設定

各 `wrangler.toml` の `zone_name` と `pattern` を自分のドメインに変更:
```toml
[[routes]]
pattern   = "あなたのドメイン.com/api/auth/*"
zone_name = "あなたのドメイン.com"
```

### Step 5: 全 Worker デプロイ

```bash
make setup       # npm install
make deploy-all  # 全 Worker をデプロイ
```

### Step 6: Cloudflare Pages にフロントエンドをデプロイ

```bash
make pages-deploy
```

または GitHub 連携（推奨）:
1. Cloudflare Dashboard → Pages → Create application
2. GitHub リポジトリを選択
3. Build command: なし (静的ファイル)
4. Output directory: `/`（ルートの index.html を配信）

### Step 7: DNS 設定

ドメインを Cloudflare に移管 or NS を Cloudflare に向けていれば自動で設定される。

---

## ローカル開発

```bash
# 各 Worker を個別に起動
make dev-auth     # http://localhost:8787
make dev-workout  # http://localhost:8788

# ローカルの D1 データベースで動作 (wrangler が自動でローカルSQLiteを作成)
curl -X POST http://localhost:8787/api/auth/register \
  -H "Content-Type: application/json" \
  -d '{"email":"test@example.com","password":"password123"}'
```

---

## コスト試算

| サービス | Free プラン | Paid ($5/月) |
|---------|------------|-------------|
| Workers | 10万リクエスト/日 | 1000万/月 |
| D1 | 5GB / 500万読み取り/日 | 25GB / 2500万/日 |
| Pages | 500ビルド/月、無制限帯域 | 同上 |
| R2 | 10GB / 100万リクエスト/月 | 10GB 超過分 $0.015/GB |
| KV | 10万読み取り/日 | 1000万/月 |

**個人利用なら Free プランでほぼ収まる。月$0〜$5。**

---

## EKS との使い分け

```
Cloudflare を選ぶケース:
  ✅ グローバル配信・低レイテンシが必要
  ✅ コストを最小化したい
  ✅ サーバー管理をゼロにしたい
  ✅ 個人・小規模プロジェクト

EKS を選ぶケース:
  ✅ K8s を本格的に学習したい
  ✅ 長時間処理 (動画変換、機械学習推論 etc.)
  ✅ Workers の CPU/メモリ制限 (10ms/128MB) を超える処理
  ✅ 既存の PostgreSQL データ資産を使いたい
  ✅ チーム開発・エンタープライズ要件
```
