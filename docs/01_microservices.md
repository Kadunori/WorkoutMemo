# マイクロサービス入門 — WorkoutMemo で学ぶ

## 1. モノリスとの比較

現在の `index.html` はすべてが1ファイルに入った **モノリス** 構成。

```
モノリス (現状)                   マイクロサービス (目標)
──────────────────────            ──────────────────────────────
index.html                        auth-service     ─ JWT 認証
  ├── UI                          workout-service  ─ トレーニング記録
  ├── 認証ロジック          →     user-service     ─ 体重・プロフィール
  ├── ワークアウト記録             menu-service     ─ 種目マスタ
  ├── 体重グラフ                   export-service   ─ CSV 出力
  └── データ: localStorage         DB: PostgreSQL (サービスごとに独立)
```

### モノリスの問題点

| 課題 | 説明 |
|------|------|
| スケール | アプリ全体しか増やせない。「ワークアウト記録だけ重い」場合も全部増やす必要がある |
| デプロイリスク | 一部変更でもアプリ全体を再起動 |
| 技術選定 | 全部同じ言語・フレームワークに縛られる |
| チーム分割 | 大きくなるとコードの干渉が増える |

---

## 2. マイクロサービスの核心原則

### ① サービスごとに独立したデータベース (DB per Service)

```
❌ 悪い例: DB 共有
  auth-service ─┐
  workout-service─┤── 共有 PostgreSQL
  user-service  ─┘

✅ 良い例: DB 分離 (このプロジェクトの設計)
  auth-service    ── auth_db
  workout-service ── workout_db
  user-service    ── user_db
  menu-service    ── menu_db
```

**なぜ分離するか?**  
- スキーマ変更が他サービスに影響しない
- サービスごとに最適な DB 種類を選べる（将来 workout は DynamoDB、menu は Redis など）

実装: [k8s/postgres/init-configmap.yaml](../k8s/postgres/init-configmap.yaml)

```sql
CREATE DATABASE auth_db;
CREATE DATABASE workout_db;
CREATE DATABASE user_db;
CREATE DATABASE menu_db;
```

---

### ② 単一責任原則

各サービスは1つのことだけを担当する。

| サービス | 責任 | API 例 |
|----------|------|--------|
| [auth-service](../services/auth-service/) | 認証・JWT発行 | POST /auth/register, POST /auth/login |
| [workout-service](../services/workout-service/) | セット記録 | GET /workouts, POST /workouts/:id/sets |
| [user-service](../services/user-service/) | プロフィール・体重 | GET /users/weight, PUT /users/profile |
| [menu-service](../services/menu-service/) | 種目マスタ | GET /menus/defaults, POST /menus |
| [export-service](../services/export-service/) | CSV生成 | GET /export/workouts |

---

### ③ API を通じた通信

サービス同士はネットワーク越しの HTTP で通信する。コードを直接呼ばない。

```
フロントエンド
    │
    ├── POST /auth/login          → auth-service
    ├── GET  /workouts            → workout-service
    ├── POST /workouts/:id/sets   → workout-service
    └── GET  /export/workouts     → export-service
                                       │
                                       ├── GET /workouts (→ workout-service)
                                       └── GET /users/weight (→ user-service)
```

export-service でのサービス間呼び出し: [services/export-service/cmd/main.go](../services/export-service/cmd/main.go)

```go
// 他サービスの API を HTTP で呼ぶ
func fetchJSON(url, token string, out any) error {
    req, _ := http.NewRequest("GET", url, nil)
    req.Header.Set("Authorization", "Bearer "+token)
    resp, _ := http.DefaultClient.Do(req)
    return json.Unmarshal(body, out)
}
```

---

## 3. JWT 認証フロー

このプロジェクトでは **各サービスが JWT を自前で検証** する（軽量・高速）。

```
① ログイン
   フロント → POST /auth/login → auth-service
                                    ↓ bcrypt でパスワード照合
                                    ↓ JWT を発行 (有効期限 24h)
   フロント ← { token: "eyJ..." }

② API 呼び出し
   フロント → GET /workouts
              Authorization: Bearer eyJ...
              → workout-service
                    ↓ JWT_SECRET で署名検証
                    ↓ user_id を取り出す
                    ↓ その user_id の記録を DB から取得
              ← [ { id: ..., muscle_group: ... } ]
```

JWT ミドルウェア（全サービス共通）: [services/workout-service/internal/middleware/jwt.go](../services/workout-service/internal/middleware/jwt.go)

```go
func JWT(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        tokenStr := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
        token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
            return []byte(secret), nil
        })
        // 検証OK → user_id をコンテキストにセット
        c.Set("user_id", claims["sub"].(string))
        c.Next()
    }
}
```

> **ポイント**: `JWT_SECRET` は全サービスが同じ値を持つ。Kubernetes では Secret リソースで管理する。

---

## 4. Clean Architecture (Go サービスの構造)

各サービスは以下のレイヤー分割で実装している。

```
cmd/main.go          ─ 起動・依存の組み立て (DI)
  │
  └── internal/
        ├── handler/   ─ HTTP リクエスト/レスポンス変換
        ├── service/   ─ ビジネスロジック
        ├── repository/─ DB アクセス
        └── model/     ─ データ構造定義
```

**依存の方向**: handler → service → repository（外側から内側へ一方通行）

例: workout-service のフロー

```
GET /workouts/:id
    ↓
handler.GetSession()         ← HTTP 解析、エラー返却
    ↓
service.GetSession()         ← セッション取得 + セット取得を合成
    ↓
repository.GetSession()      ← SELECT FROM workout_sessions WHERE id=$1
repository.ListSets()        ← SELECT FROM workout_sets WHERE session_id=$1
```

ファイル参照:
- [services/workout-service/internal/handler/workout.go](../services/workout-service/internal/handler/workout.go)
- [services/workout-service/internal/service/workout.go](../services/workout-service/internal/service/workout.go)
- [services/workout-service/internal/repository/workout.go](../services/workout-service/internal/repository/workout.go)

---

## 5. 前回重量の自動入力をマイクロサービスで実現

既存アプリの「前回重量の自動入力」機能をサーバーサイドで実装した例。

```sql
-- workout_db: 同じ種目・同じセット番号の最新記録を取得
SELECT ws.*
FROM workout_sets ws
JOIN workout_sessions sess ON sess.id = ws.session_id
WHERE sess.user_id = $1
  AND ws.exercise_name = $2
  AND ws.set_number = $3
ORDER BY ws.created_at DESC
LIMIT 1
```

API: `GET /workouts/last-set?exercise=ベンチプレス&set=1`

実装: [services/workout-service/internal/repository/workout.go](../services/workout-service/internal/repository/workout.go) の `GetLastSet`

---

## 6. マイクロサービスのトレードオフ

| メリット | デメリット |
|----------|-----------|
| 各サービスを独立スケール | サービス間通信のレイテンシ |
| 独立デプロイ・障害局所化 | 分散トランザクションが難しい |
| 技術スタックの自由度 | 運用・監視の複雑さ増加 |
| チームごとに担当分割 | ネットワーク障害への対処が必要 |

> **このプロジェクトでの判断**: 個人アプリには過剰かもしれないが、K8s学習 + 将来機能拡張（AI分析・SNS共有など）を見据えて Candidate C を選択した。
