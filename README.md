# Movie Shelf

映画を検索し、観たい・鑑賞済みの記録と評価・感想を残す日本語Webアプリです。Node.js 22以上を使用します。依存パッケージのインストールは不要です。

```sh
cp .env.example .env
npm start
```

http://localhost:3000 を開いてください。APIキー未設定でも架空の3作品で登録操作を試せます。

## API設定

`.env` に以下を設定し、サーバーを再起動してください。

- `TMDB_ACCESS_TOKEN`: TMDBアカウントの設定 → APIから取得するAPI Read Access Token、または32文字のAPIキー。値の形式から認証方式を自動で選択します。日本語タイトル検索・ポスター・人気作品に使用します。
- `TYPESAFE_API_KEY`: TypeSafe AIのAPIキー。Jevによる提案に使用します。
- `JEV_MODEL`: 既定は `jev-latest`。利用可能なモデルはTypeSafeの `/v1/models` で確認できます。

TMDBは非商用利用では出典表示を条件に無料です。商用利用条件とJevの利用料金は各サービスで確認してください。

提案では、高評価の鑑賞済み作品がある場合にTMDBの関連作品、それ以外では人気作品から未登録の最大15作品を抽出し、Jevに1本を選ばせます。候補が合わない場合は提案しません。自由文の推薦理由の生成は行いません。

登録情報はブラウザのlocalStorageに保存されます。端末間同期・アカウント・バックアップ機能はありません。提案を実行すると鑑賞済み作品のタイトル・評価・メモと気分がTypeSafe AIに送られます。APIキーはサーバーの環境変数だけで扱います。サーバーはlocalhostで起動する個人利用の試作です。

## 検証

```sh
npm test
```

## 公式資料

- [TMDB FAQ・利用条件](https://developer.themoviedb.org/docs/faq)
- [TMDB映画検索](https://developer.themoviedb.org/reference/search-movie)
- [TypeSafe API仕様](https://api.typesafe.ai/docs)
