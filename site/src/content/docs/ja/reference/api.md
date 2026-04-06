---
title: APIの例
description: 署名付きS3 URLを使ったアップロードフローとエージェント処理パイプライン。
---

## アップロードフロー

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

ステップ3の後、Inngestが`document/process.upload`を発火します。以降は割り当てられたワークフローに応じて、OCR、説明文生成、埋め込み、分岐、セグメンテーションなどが実行されます。

## 認証モデル

better-authとAPIキープラグインを使用。APIキーはorg/project/deviceにスコープされ、SHA-256ハッシュとして保存。FlutterクライアントはAPIキー検証で認証。Redisを副次的なセッションストレージとして使用。ダッシュボードはセッションベースの認証。

## ダッシュボードのドキュメントAPI

ダッシュボードの一覧/検索は次のエンドポイントを使います。

```http
GET /api/dashboard/documents?organizationId=<orgId>&query=<text>&limit=<n>&cursor=<id>
```

ポイント:

- `organizationId` は必須
- `query` 指定時は、まず埋め込み距離ベースの検索を試行し、見つからない場合は語彙検索にフォールバック
- ダッシュボード認証はセッションベース（`better-auth.session_token` Cookie、またはローカルデバッグ時の `DASHBOARD_SESSION_TOKEN`）
- レスポンス:
  - `documents`: カード配列（`id`, `objectKey`, `contentType`, `sizeBytes`, `createdAt`, `description`, `thumbnailUrl`, `distance`）
  - `nextCursor`: ページネーション用カーソル（`query` 検索時は `null`）

ダッシュボードからのアップロードは次のエンドポイントを使います。

```http
POST /api/dashboard/documents/uploads/presign
POST /api/dashboard/documents/uploads/ack
```

- `presign`: 選択したプロジェクト向けにS3直接アップロード先を発行
- `ack`: アップロードを検証し、ドキュメントを作成してダッシュボード向けイベントを発行

元ドキュメントに紐づくOCR結果の取得は次のエンドポイントです。

```http
GET /api/dashboard/documents/:id/ocr
```

- レスポンス:
  - `ocrResults`: `ocrResultId`, `ocrCreatedAt`, `modelLabel`, `text`, `avgConfidence`, `result` を持つOCR結果配列

元画像に紐づくセグメンテーション結果の取得は次のエンドポイントです。

```http
GET /api/dashboard/documents/:id/segmentations
```

- レスポンス:
  - `segmentedResults`: `segmentationId`, `segmentationCreatedAt`, `modelLabel`, `prompt`, ネストされた`document`を持つ派生画像カード

選択したダッシュボード上のドキュメントに任意の保存済みワークフローを投入するには:

```http
POST /api/dashboard/documents/:id/run
```

- Body: `{ "workflowId": "<agentGraphId>" }`
- レスポンス:
  - `status`: 常に`queued`
  - `documentId`, `workflowId`, `workflowName`

Docsタブのコレクションチャットは次のエンドポイントを使います。

```http
POST /api/documents/chat
```

ポイント:

- ダッシュボード認証はセッションベースで、アクティブな組織コンテキストに解決できる必要があります
- リクエストボディは TanStack AI のチャット形式に沿っており、`messages` に加えて `conversationId` と `scope` を任意で渡せます
- `scope` は認証済み組織の内側でなければなりません。現状のUIは組織単位で送信し、API側では必要に応じて `projectIds`、`deviceIds`、`documentIds` も受け取れます
- レスポンスは Server-Sent Events でストリーミングされます
- 根拠表示用の出典カードは `assistant_sources` カスタムイベントとして届き、`documentId`、`projectName`、任意の `deviceName`、`label`、`excerpt`、`matchReason` を含みます

## ダッシュボードのリアルタイムフィード

```http
GET /api/realtime/dashboard
```

- Server-Sent Eventsを使用
- ドキュメントイベント: `document-created`, `ocr-created`, `description-upserted`, `segmentation-created`
- 実行イベント: `run-created`, `run-step-changed`, `run-finished`

## ヘルスチェック

```
GET http://localhost:3000/health   # API
GET http://localhost:3020/health   # Agents
GET http://localhost:3021/health   # MCP
```
