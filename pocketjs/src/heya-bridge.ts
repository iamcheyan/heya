import { fetch } from "@pocketjs/framework/net";

export type BridgeCommand = "launch" | "rescan" | "get_status";
export type BridgeEvent = { id: number; type: string; app_id?: string; pid?: number };
export type BridgeStatus = { running: boolean; app_id?: string; pid?: number; started_at?: string };
export type BridgeReply<T = unknown> = { ok: boolean; id: number; apps?: T; status?: BridgeStatus; error?: string };

export async function sendBridgeCommand<T>(base: string, type: BridgeCommand, id: number, appId?: string): Promise<BridgeReply<T>> {
  const response = await fetch(`${base}/cmd`, {
    method: "POST",
    headers: { "content-type": "application/json" },
    body: JSON.stringify({ t: type, id, ...(appId ? { app_id: appId } : {}) }),
  });
  const result = await response.json() as BridgeReply<T>;
  if (!response.ok) throw new Error(result.error || `Bridge HTTP ${response.status}`);
  return result;
}

export async function pollBridgeEvents(base: string, since: number): Promise<{ next: number; events: BridgeEvent[] }> {
  const response = await fetch(`${base}/events?since=${since}`);
  if (!response.ok) throw new Error(`Bridge events HTTP ${response.status}`);
  return await response.json() as { next: number; events: BridgeEvent[] };
}
