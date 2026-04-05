---
title: アーキテクチャ
description: サービス、データパイプライン、エージェントグラフシステムの全体像。
---

## システム図

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

## パイプライン

クライアントが画像をキャプチャ → APIが署名付きS3 URLを発行 → クライアントが直接アップロード → APIが確認してInngestイベントを発火 → GoエージェントサービスがPostgresからドキュメントのエージェントグラフを読み込み → LangGraphがワークフローを構築・実行 → worker がLLMを呼び出し、tool / condition / supervisor node が処理をつなぐ → MCPがOCR、説明文、エンベディング、セグメンテーションを実行 → 結果がPostgres、ベクターインデックス、OCR結果テーブルに保存 → 意味で検索可能に。

## エージェントグラフシステム

![パイプラインとスーパーバイザーワークフローを表示するワークフローライブラリ](/dashboard-workflows.png)

エージェントグラフはコードではなくデータ。テンプレートがノード、エッジ、ツールを持つ再利用可能なワークフローを定義し、インスタンスがそれを組織に紐づけます。ノードタイプは4種類です。

- **Worker** — MCPツールにアクセスできるReActエージェント
- **Tool** — 入出力マッピング付きの単一MCPツール呼び出し
- **Supervisor** — ワーカー間のマルチエージェントオーケストレーション
- **Condition** — `contains` / `equals` でstateを判定し、true / false で次ノードを切り替える分岐ノード

すべての実行が`agent_graph_runs`と`agent_graph_run_steps`でステップごとに記録されます。OCRの生テキストと信頼度は`document_ocr_results`にも保存されるため、ダッシュボードから直接確認できます。

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

## サービスポート

| サービス | ホストポート | コンテナポート |
|-----------|-----------|----------------|
| Postgres  | 5480      | 5432           |
| Redis     | 6381      | 6379           |
| API       | 3000      | —              |
| Dashboard | 3001      | —              |
| Agents    | 3020      | —              |
| MCP       | 3021      | —              |
