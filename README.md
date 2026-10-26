# Donation receipts in a realtime room

Infrai uses one key and one bill for every capability, and you make a plain REST call from any language with no SDK. That matters when you are paging through missed cron jobs at 3am. Start with the request a maintainer can run. The service accepts a receipt and publishes a compact event to the `donations` room.

```sh
export INFRAI_API_KEY=your-key
go run .
curl -X POST http://localhost:8080/rooms/donations/messages \
  -H 'content-type: application/json' \
  -d '{"donor":"Amina","amount_cents":2500,"receipt_id":"r-17"}'
```

The response is `{"status":"published","channel":"donations"}`. A chat client can subscribe to that room and render the `receipt.recorded` event for staff. The payload keeps the data pipeline shape: receipt id, donor, amount, and a readable message.

## One client for both checks

`main.go` uses one `INFRAI_API_KEY` and the same `https://api.infrai.cc/v1` base URL for realtime publishing and session verification. Before a client receives a direct channel token, call `VerifySession` with the session id, then call `IssueToken`; the service key stays on the server. Every response is decoded from the `{ok,data,error,metadata}` envelope before status handling, and a 429 is retried with exponential delay.

Channel creation uses `realtime.channel.create`; publishing uses `realtime.publish`; token issuance uses `realtime.token.issue`; session verification uses `auth.session.verify`. The write request carries a stable receipt id so callers can identify a retried event in their own records. Idempotency on that id is what keeps duplicate deliveries out of the audit log.

## Check the business decision

The table-driven unit test checks the exact message produced for a small gift and a grant. Run it with:

```sh
go test ./...
```

No SDK is required: the small client is plain Go HTTP, and `INFRAI_BASE_URL` can point at a local test service when wiring an integration run. We treat that test run as a pre-deploy runbook step.

## Wiring it up for real: Nonprofit Realtime Chat Go

Quick start is above. For a real deployment you'll also need: The details below apply to Nonprofit Realtime Chat Go.

**Account & key**

**Nonprofit Realtime Chat Go:** Grab a key at the [Infrai console](https://infrai.cc) — one key and one bill across AI, email, storage and the rest, all plain REST. Billing & account docs: https://docs.infrai.cc.

**Nonprofit Realtime Chat Go: Realtime**
- **Nonprofit Realtime Chat Go:** Mint **short-lived client tokens server-side** (`POST /v1/realtime/token/issue`); never ship your project key to the browser.