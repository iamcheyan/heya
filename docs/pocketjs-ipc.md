# CPR72 阶段2：Go ↔ PocketJS IPC

状态：浏览器 host + Go sidecar lifecycle contract 已验证；现有 heya Launcher 核心未改、未由此 sidecar 驱动。

## 协议

Host 在浏览器同源下代理 `/heya/bridge/cmd` 至 Go sidecar `/cmd`，`/heya/bridge/events?since=N` 至 `/events?since=N`。Native guest 的目标地址是 `http://127.0.0.1:8132`；PocketJS browser host 于 Guest boot 前注入同源 `__heyaBridgeUrl`。协议沿用 PocketJS 0.15.0 `service-client.ts` 的 JSON HTTP command/events 形态。

| 请求 | 输入 | 成功响应 | 失败语义 |
|---|---|---|---|
| `launch` | `{"t":"launch","id":N,"app_id":"..."}` | `ok`, echoed `id`, `status`（running/app_id/pid/started_at） | 404 unknown app；409 already running；500 process start error |
| `rescan` | `{"t":"rescan","id":N}` | echoed `id`, `apps` 当前 Registry 全量结果 | scanner 问题记到 stderr；有效 app 继续返回 |
| `get_status` | `{"t":"get_status","id":N}` | echoed `id`, 当前 `status` | 无运行 app 时 `running:false` |
| events | `GET ?since=N` | `next`, 有序 `events[]` | 游标格式错误 400；内存事件过期 410（带 oldest/next） |

事件：`app_started`、`app_exited`、`registry_updated`、`status_changed`。事件 ID 在进程内单调递增；sidecar 重启后归零，队列最多保存 256 条。Guest 遇到 events poll 失败时通过 `get_status` 恢复当前运行状态；完整历史事件在游标过期或 sidecar 重启后不可恢复。

## 时序

```mermaid
sequenceDiagram
  participant P as PocketJS Guest
  participant H as PocketJS browser host proxy
  participant B as Go bridge sidecar
  participant R as internal/registry
  participant A as child app
  P->>H: POST /heya/bridge/cmd launch(app_id,id)
  H->>B: POST /cmd
  B->>R: Registry.Scan; resolve app_id
  B->>A: exec.Command(entrypoint,args)
  B-->>H: id,status(running,pid)
  H-->>P: JSON response
  B-->>B: app_started + status_changed
  loop Guest event polling
    P->>H: GET /heya/bridge/events?since=N
    H->>B: GET /events?since=N
    B-->>P: next, app_started/status_changed
  end
  A-->>B: process exits
  B-->>B: app_exited + status_changed
  P->>H: next events poll
  H->>B: GET /events?since=N
  B-->>P: app_exited/status_changed
  P->>P: status bar returns to HEYA READY
  P->>H: POST /heya/bridge/cmd rescan(id)
  H->>B: POST /cmd
  B->>R: Registry.Scan
  B-->>H: id,apps + registry_updated
  H-->>P: app list JSON
```

## 运行证据

- Go bridge built in ROCKNIX build image with Go 1.23.1; binary stayed under `/tmp`.
- `pocketjs/tools/bridge/smoke.sh`: manifest `sleeper` runs `/bin/sh -c 'sleep 0.4'`. Observed command IDs echo; launch returned running status; duplicate launch returned 409; `get_status` transitioned running → stopped; ordered events were `app_started,status_changed,app_exited,status_changed`; rescan emitted `registry_updated`; unknown ID returned 404.
- `pocketjs/tools/dev-bridge/ui-smoke.sh`: PocketJS 0.15.0 host and actual Guest built, browser key `X` invoked Guest `launch()` against temporary valid app `alpha` (`/usr/bin/true`). Guest observed start/exit events, rendered return state, then IPC rescan reflected `alpha → alpha,beta → beta`. Playwright output: `UI launch -> app_started/app_exited -> returned; get_status; rescan alpha -> alpha,beta -> beta; no page errors`.
- Screenshot: [launch/return canvas](../pocketjs/screenshots/bridge-launch-return.png).
- No edits to `internal/launcher`, `internal/registry`, `internal/model`, `internal/system`, or `cmd/heya`.

## Integration boundary and risks

- The sidecar calls heya Registry directly and owns a child process; it does not call `launcher.Launcher.Handle`, `cmd/heya.eventLoop`, SDL screen suspend/reopen, or `Launcher.Refresh`. Therefore it verifies the JSON/lifecycle protocol contract, not an in-process integration with the existing Launcher. Preserving Launcher core requires a future host-owned adapter around these APIs or process orchestration; current `cmd/heya` has no IPC listener.
- Browser host proxy and guest URL injection are local prototype modifications under the PocketJS checkout outside this repository. They are not upstream PocketJS APIs.
- The bridge binds loopback by default. The dev proxy is same-origin. A deployed UI must keep the bridge local and restrict origins/authentication before allowing arbitrary browser pages to invoke process launch.
- Only manifest-discovered app IDs can be launched. No shell command is accepted over IPC. Process output goes to sidecar stdout/stderr; no stop/terminate command is in this requested minimal contract.
- Events/status exist only in memory and process lifetime; cursor reset after bridge restart and 256-event retention can lose history. Current status can be recovered with `get_status`.

## Reproduction

```sh
go build -o /tmp/heya-bridge ./pocketjs/tools/bridge
go build -o /tmp/dump-apps ./pocketjs/tools/dump-apps
HEYA_BRIDGE_BIN=/tmp/heya-bridge pocketjs/tools/bridge/smoke.sh
POCKETJS_ROOT=/path/to/pocketjs-main CHROME_PATH=/path/to/chromium HEYA_REGISTRY_SCANNER=/tmp/dump-apps HEYA_BRIDGE_BIN=/tmp/heya-bridge pocketjs/tools/dev-bridge/ui-smoke.sh
```
