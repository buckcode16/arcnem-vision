<p align="center">
  <img src="arcnem-logo.svg" alt="Arcnem" width="120" />
</p>

<h1 align="center">Arcnem Vision</h1>

<p align="center">
  <strong>機械に「見る」を教え、エージェントに「どうするか」を任せる。</strong>
</p>

<p align="center">
  <a href="README.md">English</a> ·
  <a href="#クイックスタート">クイックスタート</a> ·
  <a href="#アーキテクチャ">アーキテクチャ</a> ·
  <a href="site/">ドキュメントサイト</a> ·
  <a href="docs/">ディープダイブ</a>
</p>

---

Arcnem Visionは、画像を「理解」に変えるオープンソースプラットフォームです。Flutterアプリからでも、ダッシュボードからでも写真をアップロードでき、LangGraphでオーケストレーションされ、MCPで接続され、データベースから設定されたAIエージェント群が、OCR抽出、エンベディング生成、説明文作成、条件ノードやスーパーバイザーによる分岐、セグメンテーション実行、そしてメタデータではなく意味による検索まで担います。

4つの言語。5つのサービス。カメラのシャッターからセマンティック検索まで、1本のパイプライン。

> **必要なAPIキーはたった2つ。** [OpenAI APIキー](https://platform.openai.com/api-keys)と[Replicate APIトークン](https://replicate.com/account/api-tokens)を取得するだけ。Postgres、Redis、S3、Inngestなど、その他すべては`docker compose`でローカル実行。

**ここが面白い：**

- **データベース駆動のエージェントグラフ** — AIワークフローをコードではなく行として定義。worker / tool / supervisor / condition を組み合わせた処理パイプラインを、再デプロイなしで組織ごとに切り替え可能。
- **GenUIチャットインターフェース** — AIはテキストで返答するだけではない。カード、ギャラリー、インタラクティブコンポーネントなど、実際のFlutterウィジェットをJSONからランタイムで生成。
- **オンデバイスGemma** — インテント解析はネットワークに到達する前にスマートフォン上でローカル実行。デフォルトでプライバシー重視。
- **CLIPベクトル検索** — 画像とその説明文を同一の768次元空間に埋め込み。画像で検索、テキストで検索、雰囲気で検索。
- **ダッシュボード運用画面** — プロジェクト、デバイス、APIキー、ワークフロー割り当て、単発のダッシュボードアップロードを同じUIで管理。
- **ビジュアルワークフロービルダー** — worker、tool、supervisor、condition node、edge を組み合わせて、再利用可能なエージェントグラフを視覚的に構築。
- **OCR前提のドキュメント運用** — OCRはMCPツールとして扱われ、抽出テキストと信頼度を保存。その結果を条件分岐にも、専門ワーカーへの振り分けにも使えます。
- **リアルタイムの運用フィードバック** — DocsタブとRunsタブが、アップロード、OCR結果保存、説明文生成、セグメンテーション、実行ステップの進行に合わせて更新。
- **MCPツールがファーストクラス** — CLIPエンベディング、説明文生成、OCR、類似検索、セグメンテーションモデルをMCPの背後に集約。エージェントも人も利用可能。

## 技術スタック

| レイヤー           | 技術                                           | 役割                                                                                      |
| ------------------ | ---------------------------------------------- | ----------------------------------------------------------------------------------------- |
| **クライアント**   | Flutter, Dart, flutter_gemma, GenUI, fpdart    | カメラキャプチャ、オンデバイスLLM、AI生成UI、関数型エラーハンドリング                     |
| **API**            | Bun, Hono, better-auth, Inngest, Pino          | RESTルート、署名付きアップロード、ジョブスケジューリング、構造化ログ                      |
| **ダッシュボード** | React 19, TanStack Router, Tailwind, shadcn/ui | ワークフロービルダー、プロジェクト/デバイス/APIキー管理、ライブ運用UI                       |
| **エージェント**   | Go, Gin, LangGraph, LangChain, inngestgo       | グラフベースのエージェントオーケストレーション、ReActワーカー、ステップレベルトレーシング |
| **MCP**            | Go, MCP go-sdk, replicate-go, GORM             | CLIPエンベディング、説明文生成、OCR、セグメンテーション、類似検索                         |
| **ストレージ**     | Postgres 18 + pgvector, S3互換, Redis          | ベクターインデックス、オブジェクトストレージ、セッションキャッシュ                        |

## アーキテクチャ

```
┌─────────────┐     ┌──────────────┐     ┌─────────────────┐
│   Flutter    │────▶│   Hono API   │────▶│     Inngest     │
│   Client     │     │   (Bun)      │     │   Event Queue   │
│              │     │              │     │                 │
│ GenUI + Gemma│     │ Presigned S3 │     └────────┬────────┘
└─────────────┘     │ better-auth  │              │
                    └──────────────┘              ▼
┌─────────────┐                        ┌──────────────────┐
│    React     │                        │   Go Agents      │
│  Dashboard   │                        │                  │
│              │     ┌──────────┐       │ LangGraph loads  │
│  Workflow    │────▶│ Postgres │◀──────│ graph from DB,   │
│  Builder     │     │ pgvector │       │ executes nodes   │
└─────────────┘     └──────────┘       └────────┬─────────┘
                         ▲                      │
                         │               ┌──────▼─────────┐
                    ┌────┴───┐           │   MCP Server    │
                    │   S3   │           │                 │
                    │Storage │           │ OCR, CLIP,      │
                    └────────┘           │ Descriptions,   │
                                         │ Search, Segment │
                                         └─────────────────┘
```

**パイプライン：** クライアントが画像をキャプチャ → APIが署名付きS3 URLを発行 → クライアントが直接アップロード → APIが確認してInngestイベントを発火 → GoエージェントサービスがPostgresからドキュメントのエージェントグラフを読み込み → LangGraphがワークフローを構築・実行 → worker がLLMを呼び出し、tool / condition / supervisor node が処理をつなぐ → MCPがOCR、説明文、エンベディング、セグメンテーションを実行 → 結果がPostgresとベクターインデックス、OCR結果テーブルに保存 → 意味で検索可能に。

## スクリーンショット

| Flutterクライアント | ダッシュボード — プロジェクト＆デバイス |
|---|---|
| ![Flutterクライアント](site/public/flutter-client.png) | ![ダッシュボード プロジェクト](site/public/dashboard-projects.png) |

| ワークフローライブラリ | Docsギャラリー |
|---|---|
| ![ワークフローライブラリ](site/public/dashboard-workflows.png) | ![Docsギャラリー](site/public/dashboard-docs.png) |

| 選択中のドキュメントとセグメンテーション | エージェント実行詳細 |
|---|---|
| ![選択中のドキュメントとセグメンテーション](site/public/dashboard-docs-segmentation-detail.png) | ![エージェント実行詳細](site/public/dashboard-run-detail.png) |

**エージェントグラフはコードではなくデータ。** テンプレートがノード、エッジ、ツールを持つ再利用可能なワークフローを定義。インスタンスがテンプレートを組織にバインド。ノードタイプは4種類：

- **Worker** — MCPツールにアクセスできるReActエージェント
- **Tool** — 入出力マッピング付きの単一MCPツール呼び出し
- **Supervisor** — ワーカー間のマルチエージェントオーケストレーション
- **Condition** — stateを`contains` / `equals`で判定し、true / false で次ノードを切り替える決定論的な分岐

すべての実行が`agent_graph_runs`と`agent_graph_run_steps`でステップごとにトレースされます。さらにOCRの生テキストと信頼度は`document_ocr_results`に別保存されるため、Runsを開かなくてもDocs画面から確認できます。

## リポジトリ構成

```
arcnem-vision/
├── client/                 Flutterアプリ — GenUI、Gemma、カメラ、ギャラリー
│   ├── lib/screens/        認証、カメラ、ダッシュボード、ローディング
│   ├── lib/services/       アップロード、ドキュメント、GenUI、インテント解析
│   └── lib/catalog/        AI生成UIのためのカスタムウィジェットカタログ
├── server/                 Bunワークスペース
│   ├── packages/api/       Honoルート、ミドルウェア、認証、S3、Inngest
│   ├── packages/db/        Drizzleスキーマ（23テーブル）、マイグレーション、シード
│   ├── packages/dashboard/ React管理画面 — ワークフロービルダー、ドキュメントビューア
│   └── packages/shared/    Envヘルパー
├── models/                 Goワークスペース
│   ├── agents/             Inngestハンドラー、LangGraph実行エンジン
│   ├── mcp/                MCPサーバー — 7ツール（説明文、OCR、埋め込み、セグメンテーション、検索）
│   ├── db/                 GORM genイントロスペクション（スキーマ → Goモデル）
│   └── shared/             共通env読み込み
└── docs/                   ディープダイブ — エンベディング、LangChain、LangGraph、GenUI
```

## クイックスタート

### 1. クローンと設定

```bash
git clone https://github.com/arcnem-ai/arcnem-vision.git
cd arcnem-vision
```

すべての`.env.example`を`.env`にコピー：

```bash
cp server/packages/api/.env.example server/packages/api/.env
cp server/packages/db/.env.example  server/packages/db/.env
cp models/agents/.env.example       models/agents/.env
cp models/mcp/.env.example          models/mcp/.env
cp client/.env.example              client/.env
```

外部サービスで必要なのはAPIキー2つだけ：

- **[OpenAI APIキー](https://platform.openai.com/api-keys)** → `models/agents/.env`に`OPENAI_API_KEY`
- **[Replicate APIトークン](https://replicate.com/account/api-tokens)** → `models/mcp/.env`に`REPLICATE_API_TOKEN`

それ以外はすべてローカル開発用に設定済み。データベース、S3、Redisは`docker-compose.yaml`のDockerで起動され、`.env.example`のデフォルト値がそのまま使えます。

### 2. すべてを起動

```bash
tilt up
```

これだけです。Tiltがすべての依存関係をインストールし、Postgres/Redis/MinIOを起動し、マイグレーションを実行し、すべてのサービス（API、ダッシュボード、エージェント、MCP、Inngest、Flutterクライアント、ドキュメントサイト）を起動します。Tilt UI（`http://localhost:10350`）でログの確認やリソースの管理ができます。

### 3. データベースのシード

Tilt UIで**seed-database**リソースをクリックし、トリガーボタンを押します。シードでは、デモ用の組織、プロジェクト、デバイス、APIキー、新しいサンプル画像、OCRの条件分岐ワークフロー、OCRレビュー用スーパーバイザーワークフロー、そしてセグメンテーションのショーケース用ワークフローが作成されます。さらに、使用可能なAPIキーも出力されます。開発中のFlutterアプリで自動認証するには、`client/.env`に`DEBUG_SEED_API_KEY=...`を設定してください。

### ヘルスチェック

```
GET http://localhost:3000/health   # API
GET http://localhost:3020/health   # Agents
GET http://localhost:3021/health   # MCP
```

### S3設定の詳細

ローカル開発のデフォルトは`docker-compose.yaml`のMinIOを使用。`.env.example`ファイルに動作するデフォルト値が設定済み：

- `S3_ACCESS_KEY_ID=minioadmin`
- `S3_SECRET_ACCESS_KEY=minioadmin`
- `S3_BUCKET=arcnem-vision`
- `S3_ENDPOINT=http://localhost:9000`
- `S3_REGION=us-east-1`
- `S3_USE_PATH_STYLE=true`（agentsのみ）

ホスト型ストレージの場合は、AWS S3 / Cloudflare R2 / Railway Object Storage / Backblaze B2の認証情報に置き換えてください。

## APIの例

```bash
# 1. 署名付きアップロードURLを取得
curl -X POST http://localhost:3000/api/uploads/presign \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${API_KEY}" \
  -d '{"contentType":"image/png","size":12345}'

# 2. 返されたuploadUrlでS3に直接アップロード
curl -X PUT "${UPLOAD_URL}" --data-binary @photo.png

# 3. 確認 — エージェントパイプライン全体がトリガーされる
curl -X POST http://localhost:3000/api/uploads/ack \
  -H "Content-Type: application/json" \
  -H "x-api-key: ${API_KEY}" \
  -d '{"objectKey":"uploads/.../photo.png"}'
```

ステップ3の後、Inngestが`document/process.upload`を発火。以降は割り当てられたワークフロー次第で、OCR、説明文生成、埋め込み、分岐、ベクターインデックス作成まで進みます。

## 必要条件

- Docker + Docker Compose
- Bun（サーバー）
- Go 1.25+（エージェント、MCP）
- CompileDaemon（`go install github.com/githubnemo/CompileDaemon@latest`）
- Flutter SDK（クライアント）
- Tilt

## ドキュメント

| ドキュメント                               | 内容                                                                                   |
| ------------------------------------------ | -------------------------------------------------------------------------------------- |
| [site/](site/)                             | オンボーディングと参照用のローカルドキュメントサイト（Starlight）                      |
| [docs/embeddings.md](docs/embeddings.md)   | 現在のエンベディング実装と運用上の制約                                                 |
| [docs/langgraphgo.md](docs/langgraphgo.md) | グラフオーケストレーションパターン、condition node、supervisor routing、チェックポイント |
| [docs/langchaingo.md](docs/langchaingo.md) | LLMプロバイダー、チェーン、エージェント、ツール、MCPブリッジ                           |
| [docs/genui.md](docs/genui.md)             | Flutter GenUI SDK、DataModelバインディング、A2UIプロトコル、カスタムウィジェット       |

## コントリビューション

[CONTRIBUTING.md](CONTRIBUTING.md) にコントリビューション手順があります。AIコーディングエージェントを使う場合は [AGENTS.md](AGENTS.md) も参照してください。

---

<p align="center">
  東京の<a href="https://arcnem.ai">Arcnem AI</a>が開発。
</p>
