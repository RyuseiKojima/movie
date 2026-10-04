# Movie Shelf

映画を検索し、観たい・鑑賞済みの記録と評価・感想を残す日本語Webアプリです。フロントエンドはReact + TypeScript（Vite）、バックエンドはGoの標準ライブラリのみで構成しています。フロントエンドの開発・ビルドにはNode.js 22以上、バックエンドにはGo 1.25以上が必要です。

```sh
cp .env.example .env
npm install
npm run build
npm start
```

http://localhost:3000 を開いてください。APIキー未設定でも架空の3作品で登録操作を試せます。

### 開発

`npm start` でAPIサーバーを起動したまま、別のターミナルで `npm run dev` を実行するとViteの開発サーバー（http://localhost:5173 ）が起動します。`/api` へのリクエストはポート3000のサーバーへプロキシされます。

- `src/` — Reactコンポーネント（`components/`）、localStorage連携（`hooks/useLibrary.ts`）、API呼び出し（`api.ts`）、型定義（`types.ts`）
- `backend/` — GoのHTTPサーバー、TMDB・Jev連携、設定読み込みとテスト
- `go.mod` — Goモジュール定義（外部依存なし）

### バックエンドのビルドと実行

```sh
npm run build:server
./bin/movie-shelf
```

実行時の作業ディレクトリにある `.env` と `dist/` を使用します。別の場所を使う場合は `./bin/movie-shelf -env /path/to/.env -static /path/to/dist` で指定できます。ポートは `.env` または環境変数の `PORT` で設定でき、既定値は3000です。バックエンドのバイナリ実行にはNode.jsは不要です。

`server.js` はGoへ置き換えました。`/api/config`、`/api/search`、`/api/recommend` のURLとJSON形式は共通で、ブラウザに保存したライブラリは引き続き利用できます。

## API設定

`.env` に以下を設定し、サーバーを再起動してください。Goサーバーが起動時に読み込みます。既存の環境変数を優先し、未指定の変数だけを補完します。空行・コメント・単一行の引用符付きの値・`export KEY=value`に対応します。

- `TMDB_ACCESS_TOKEN`: TMDBアカウントの設定 → APIから取得するAPI Read Access Token、または32文字のAPIキー。値の形式から認証方式を自動で選択します。日本語タイトル検索・ポスター・人気作品に使用します。
- `TYPESAFE_API_KEY`: TypeSafe AIのAPIキー。Jevによる提案に使用します。
- `JEV_MODEL`: 既定は `jev-latest`。利用可能なモデルはTypeSafeの `/v1/models` で確認できます。

TMDBは非商用利用では出典表示を条件に無料です。商用利用条件とJevの利用料金は各サービスで確認してください。

提案では、「邦画」「日本映画」「日本の映画」を希望する場合は日本製作・日本語の作品、それ以外は高評価の鑑賞済み作品がある場合にTMDBの関連作品、ない場合は人気作品から未登録の最大15作品を抽出し、Jevに1本を選ばせます。候補が合わない場合は提案しません。自由文の推薦理由の生成は行いません。

登録情報はブラウザのlocalStorageに保存されます。端末間同期・アカウント・バックアップ機能はありません。提案を実行すると鑑賞済み作品のタイトル・評価・メモと気分がTypeSafe AIに送られます。APIキーはサーバーの環境変数だけで扱います。サーバーはlocalhostで起動する個人利用の試作です。

## 検証

```sh
npm test
```

型チェックとフロントエンドのビルドを実行してから、Goのテストを競合検出付きで実行します。外部APIはテスト内で模擬するため、APIキーは不要です。バックエンドのみの検証は `go test -race ./...` と `go vet ./...` で実行できます。

## 公式資料

- [TMDB FAQ・利用条件](https://developer.themoviedb.org/docs/faq)
- [TMDB映画検索](https://developer.themoviedb.org/reference/search-movie)
- [TypeSafe API仕様](https://api.typesafe.ai/docs)

- [Go HTTP標準ライブラリ](https://pkg.go.dev/net/http)
