# MMate (WorkoutMemo)

筋トレ中の疲労した状態でも簡単に記録できるトレーニング記録アプリ。

## 構成

```
web/        アプリ本体(単一HTML・依存なし・localStorage保存)
ios/        iOSネイティブアプリ(Capacitor生成)
android/    Androidネイティブアプリ(Capacitor生成)
```

`web/index.html` が唯一のソース。Web(Cloudflare)とiOS/Androidネイティブアプリで共有する。

## Webのデプロイ(Cloudflare Pages)

GitHub Pages は廃止し、Cloudflare Pages で運用する。

### 初回セットアップ(どちらか)

**A. Git連携(推奨)** — Cloudflareダッシュボード → Workers & Pages → Create → Pages →
「Connect to Git」で本リポジトリを選択。

- Build command: (なし)
- Build output directory: `web`

以後 `main` への push で自動デプロイされる。

**B. CLIで手動デプロイ**

```sh
npx wrangler login
npm run deploy:web
```

## ネイティブアプリ(Capacitor)

`web/` を編集したら各プラットフォームへ同期する:

```sh
npm install        # 初回のみ
npm run sync       # web/ → ios/ + android/ へコピー
```

### iOS(要: Xcode)

```sh
npm run ios        # Xcodeでios/App/App.xcodeprojを開く
```

Xcodeで Signing(Apple IDのTeam)を設定して実機/シミュレータで Run。
App Store配布には Apple Developer Program($99/年)が必要。

### Android(要: Android Studio)

```sh
npm run android    # Android Studioでandroid/を開く
```

Android StudioでそのままRun。配布APK/AABは Build → Generate Signed Bundle で作成。

- App ID: `com.kadunori.mmate`
- データはWebViewのlocalStorageに保存(端末内で完結、サーバー不要)
