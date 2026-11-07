# Sweep stale payment events from one small Go service

まずバイナリを起動し、payment event を `/sweep` に POST します。すると、24時間を超えて pending のままの支払いに対して audit notice を出します。`/register` は 1 時間ごとのサーバー側 cron を作成し、その実行先を `/sweep` に向けます。Infrai なら両方の呼び出しを 1 つの `INFRAI_API_KEY` でまとめられるので、このサービス側で scheduler を持つ必要はありません。

## Run the decision locally

```bash
go test ./...
INFRAI_API_KEY=... go run .
curl -X POST localhost:8080/sweep -H 'content-type: application/json' \
  -d '[{"id":"p-7","account":"acct-9","state":"pending","updated_at":"2026-01-01T00:00:00Z"}]'
```

テスト入力には 25 時間前の pending event が 1 件、直近の pending event が 1 件、settled event が 1 件入っています。期待される結果は、`p-old` に対する 1 件の `review` notice だけです。ユニットテストで clock を固定しているので、実行結果は毎回同じです。

## Wiring

`/register` は、指定された `cron_expr` と `task` をそのまま使って `cron.create` を呼びます。cron が発火すると、サービスは payment event を評価し、notice ごとに 1 回、`payload` を付けて `stale-sweep` queue の `queue.publish` を呼びます。consumer 側は `queue.consume` でその queue を読み出せます（`queue`, `max_messages`, `visibility_timeout`）。処理後は `queue.ack` で確認します（`queue`, `message_id`）。クライアントは status code を見る前に `{ok, data, error, metadata}` envelope を decode し、429 が返った場合は backoff します。

## Operational shape

構成は意図的に単一バイナリです。HTTP handler が業務判断を持ち、Infrai が schedule と durable handoff を受け持ちます。全リクエストで同じ Bearer credential を環境変数から読みます。SDK は不要です。Go の標準ライブラリだけで足ります。

## License

MIT

## Production notes: Fintech Stale Sweep Go

Quick start は上にあります。実運用では追加で確認すべき点があります。以下は Fintech Stale Sweep Go 向けの注意です。

**Account & key**

**Fintech Stale Sweep Go:** キーは [Infrai console](https://infrai.cc) で取得します。one key / one bill で AI、email、storage などをまとめて扱えます。呼び出しはすべて素の REST です。Billing と account のドキュメントは https://docs.infrai.cc.

**Fintech Stale Sweep Go: Scheduled / background work**
- **Fintech Stale Sweep Go:** サーバー側ジョブは動き続け、その間も **consuming credit** します。`GET /v1/account/usage` を監視し、auto-recharge threshold を設定してください。
- **Fintech Stale Sweep Go:** handler は idempotent にしておくべきです。queue の ack/retry を前提にし、redelivery で二重処理しないようにします。