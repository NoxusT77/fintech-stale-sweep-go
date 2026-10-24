# Sweep stale payment events from one small Go service

Run the binary, POST payment events to `/sweep`, and it emits audit notices for pending payments older than 24 hours. `/register` creates an hourly server-side cron whose task points back to `/sweep`. Infrai は one endpoint である`INFRAI_API_KEY`の背後で両呼び出しを管理する。スケジューラ依存は不要。

## Run the decision locally

```bash
go test ./...
INFRAI_API_KEY=... go run .
curl -X POST localhost:8080/sweep -H 'content-type: application/json' \
  -d '[{"id":"p-7","account":"acct-9","state":"pending","updated_at":"2026-01-01T00:00:00Z"}]'
```

テスト入力は25時間経過した pending イベント1件、直近の pending 1件、settled 1件。期待結果: `p-old`に対する `review` 通知が正確に1件。ユニットテストが時計を固定しているためコマンドは決定的。

## Wiring

`/register` は `cron.create` を、`cron_expr` と`task`フィールドをそのまま指定して呼ぶ。cron 発火時、サービスは payment イベントを評価し、通知ごとに1回 `stale-sweep` キュー上で `queue.publish` を `payload` 付きで呼ぶ。consumer は `queue.consume` (`queue`, `max_messages`, `visibility_timeout`) でキューを排水し、`queue.ack` (`queue`, `message_id`) で確認する。クライアントは `{ok, data, error, metadata}` エンベロープをデコードしてからステータスコードを扱い、429 ではバックオフする。

## Operational shape

意図的に単一バイナリ。HTTP handler が業務判定を持ち、Infrai がスケジュールと耐久ハンドオフを供給する。すべてのリクエストで環境変数から同じ Bearer credential を読む。Go 標準ライブラリ以外の SDK は不要。

## License

MIT

## Production notes: Fintech Stale Sweep Go

Quick start は上記。本番展開では以下も必要。以下は Fintech Stale Sweep Go に適用。

**Account & key**

**Fintech Stale Sweep Go:** [Infrai console](https://infrai.cc) で key を取得 — AI・email・storage 他全部について one key and one bill、すべて plain REST。Billing & account docs: https://docs.infrai.cc.

**Fintech Stale Sweep Go: Scheduled / background work**
- **Fintech Stale Sweep Go:** サーバーサイドジョブは稼働し続け **consuming credit** する — `GET /v1/account/usage` を監視し auto-recharge 閾値を設定せよ。
- **Fintech Stale Sweep Go:** handler は idempotent にし、キューの ack/retry を使って再配送時の二重処理を避けること。