import { mount } from "@pocketjs/framework/solid";
import { Focusable, FocusGrid, FocusScope, View } from "@pocketjs/framework/solid/components";
import { BTN, getFocused, pushFocusScope, focusNode } from "@pocketjs/framework/input";
import type { NodeMirror } from "@pocketjs/framework/solid/renderer";
import { onButtonPress } from "@pocketjs/framework/solid/lifecycle";
import { spring } from "@pocketjs/framework/animation";
import { createEffect, createSignal, For, onCleanup, onMount } from "solid-js";
import { toPocketCards, type HeYaApp, type PocketCard } from "./heya-registry";
import { pollBridgeEvents, sendBridgeCommand, type BridgeStatus } from "./heya-bridge";
import { scannedApps } from "./apps.generated";

const bridgeUrl = (globalThis as typeof globalThis & { __heyaBridgeUrl?: string }).__heyaBridgeUrl ?? "http://127.0.0.1:8132";

type ScannedApp = (typeof scannedApps)[number];
type App = PocketCard & { glyph: string };
const toApps = (records: HeYaApp[]): App[] => toPocketCards(records).map((card) => ({
  ...card,
  category: card.category.toUpperCase(),
  subtitle: card.subtitle || "Discovered by the heya registry",
  glyph: card.name.trim().slice(0, 1).toUpperCase(),
}));
const demoApps = toApps(scannedApps as ScannedApp[]);
function App() {
  const [apps, setApps] = createSignal(demoApps);
  const [index, setIndex] = createSignal(demoApps.length > 1 ? 1 : 0);
  const [favorite, setFavorite] = createSignal(false);
  const [menuOpen, setMenuOpen] = createSignal(false);
  const [clock, setClock] = createSignal("09:41");
  const [message, setMessage] = createSignal("A quiet place for the things you use every day");
  const [runtime, setRuntime] = createSignal<BridgeStatus>({ running: false });
  let requestId = 0;
  let eventCursor = 0;
  let refreshing = false;
  let polling = false;
  let timer = 0;
  let menuRoot: NodeMirror | undefined;
  let previousFocus: NodeMirror | undefined;
  let cardNodes: Array<{ id: number } | undefined> = [];
  const rescan = async () => {
    if (refreshing) return;
    refreshing = true;
    try {
      const result = await sendBridgeCommand<ScannedApp[]>(bridgeUrl, "rescan", ++requestId);
      const next = toApps(result.apps ?? []);
      if (!next.length) throw new Error("Registry returned no visible applications");
      setApps(next);
      setIndex((current) => Math.min(current, next.length - 1));
      setMessage(`Registry refreshed · ${next.length} applications`);
    } catch (error) {
      setMessage(`Registry refresh failed · ${String(error)}`);
    } finally {
      refreshing = false;
    }
  };
  const refreshStatus = async () => {
    const result = await sendBridgeCommand(bridgeUrl, "get_status", ++requestId);
    if (result.status) setRuntime(result.status);
  };
  const pollEvents = async () => {
    if (polling) return;
    polling = true;
    try {
      const batch = await pollBridgeEvents(bridgeUrl, eventCursor);
      eventCursor = batch.next;
      for (const event of batch.events) {
        if (event.type === "app_started") {
          setRuntime({ running: true, app_id: event.app_id, pid: event.pid });
          setMessage(`${event.app_id ?? "Application"} started · waiting for exit`);
        } else if (event.type === "app_exited") {
          setRuntime({ running: false });
          setMessage(`${event.app_id ?? "Application"} exited · returned to heya`);
        }
      }
    } catch {
      try { await refreshStatus(); } catch { setMessage("heya bridge unavailable"); }
    } finally {
      polling = false;
    }
  };
  const move = (delta: number) => {
    setIndex((current) => (current + delta + apps().length) % apps().length);
    requestAnimationFrame(() => {
      const node = cardNodes[index()];
      if (node) {
        const focused = getFocused();
        if (focused) spring(focused, "scale", 1.06, "bouncy");
      }
    });
  };
  const active = () => apps()[index()];
  const launch = async () => {
    if (runtime().running) return setMessage(`${runtime().app_id ?? "Application"} is already running`);
    try {
      const result = await sendBridgeCommand(bridgeUrl, "launch", ++requestId, active().id);
      if (result.status) setRuntime(result.status);
      setMessage(`${active().name} launched`);
      void pollEvents();
    } catch (error) {
      setMessage(`Launch failed · ${String(error)}`);
    }
  };
  const toggleFavorite = () => { const next = !favorite(); setFavorite(next); setMessage(`${active().name} ${next ? "added to" : "removed from"} favorites`); };
  createEffect(() => {
    if (menuOpen() && menuRoot) {
      previousFocus = getFocused() ?? undefined;
      const dispose = pushFocusScope(menuRoot, { autoFocus: true, restoreFocus: false });
      onCleanup(() => { dispose(); if (previousFocus) focusNode(previousFocus); });
    }
  });
  onMount(() => {
    void rescan();
    void refreshStatus().then(pollEvents).catch(() => setMessage("heya bridge unavailable"));
    timer = window.setInterval(() => void pollEvents(), 1_000);
    onButtonPress(BTN.LEFT, () => move(-1));
    onButtonPress(BTN.RIGHT, () => move(1));
    onButtonPress(BTN.CROSS, () => void launch());
    onButtonPress(BTN.TRIANGLE, toggleFavorite);
    onButtonPress(BTN.SQUARE, () => setMenuOpen((value) => !value));
    onButtonPress(BTN.CIRCLE, () => setMenuOpen(false));
  });
  onCleanup(() => window.clearInterval(timer));
  const offset = (i: number) => { let d = i - index(); const n = apps().length; if (d > n / 2) d -= n; if (d < -n / 2) d += n; return d; };
  return (
    <View class="flex-col w-full h-full bg-slate-950 p-3">
      <View class="flex-row h-10 items-center justify-between">
        <View class="text-xl text-white font-bold">heya.</View>
        <View class="flex-row gap-4 text-sm text-slate-300"><View>{runtime().running ? `RUNNING ${runtime().app_id}` : "HEYA READY"}</View><View>Wi-Fi</View><View>Battery</View><View>{clock()}</View></View>
      </View>
      <View class="flex-row h-8 items-center justify-between text-xs text-slate-400"><View>YOUR POCKET, IN FLOW</View><View>{String(index() + 1).padStart(2, "0")} / {apps().length}</View></View>
      <View class="flex-1 flex-row items-center justify-center gap-2">
        <FocusGrid columns={1} wrap={true} class="flex-row items-center justify-center gap-2">
          <For each={apps()}>{(app, i) => {
            const d = () => offset(i());
            return <Focusable class={d() === 0 ? "w-[170] h-[190] flex-col justify-between p-3 rounded-xl border border-amber-400 bg-slate-800 focus:bg-slate-700 transition-transform duration-300 focus:scale-105" : "w-[140] h-[160] flex-col justify-between p-2 rounded-xl border border-slate-700 bg-slate-900 focus:bg-slate-800 transition-transform duration-300"} style={{ translateX: d() * 18, scale: d() === 0 ? 1 : 0.85, opacity: Math.abs(d()) > 1 ? 0.45 : 1 }} ref={(node) => { cardNodes[i()] = node; }} onPress={() => { setIndex(i()); launch(); }}>
              <View class="flex-row justify-between text-xs text-slate-400"><View>{app.category}</View><View>*</View></View>
              <View class="flex-col items-center justify-center gap-3"><View class="text-2xl text-amber-300">{app.glyph}</View><View class="text-lg text-white font-bold">{app.name}</View></View>
              <View class="text-xs text-slate-300">{app.subtitle}</View>
            </Focusable>;
          }}</For>
        </FocusGrid>
      </View>
      <View class="flex-col items-stretch gap-1 text-xs text-slate-300">
        <View>{message()}</View>
        <View class="flex-row items-center justify-between gap-1"><Focusable class="rounded-md bg-slate-800 px-1 py-1" onPress={launch}><View>A Open</View></Focusable><Focusable class="rounded-md bg-slate-800 px-1 py-1" onPress={() => setMenuOpen(false)}><View>B Back</View></Focusable><Focusable class="rounded-md bg-slate-800 px-1 py-1" onPress={() => setMenuOpen((v) => !v)}><View>X Menu</View></Focusable><Focusable class="rounded-md bg-slate-800 px-1 py-1" onPress={toggleFavorite}><View>Y {favorite() ? "Saved" : "Favorite"}</View></Focusable></View>
      </View>
      {menuOpen() && <View class="absolute right-4 bottom-16 w-[260] flex-col gap-3 p-4 rounded-xl border border-slate-600 bg-slate-800" ref={(node) => { menuRoot = node; }}><View class="text-sm text-white font-bold">QUICK MENU</View><Focusable onPress={toggleFavorite}><View class="text-sm text-white">☆ {favorite() ? "Remove favorite" : "Add to favorites"}</View></Focusable><Focusable onPress={() => { void rescan(); setMenuOpen(false); }}><View class="text-sm text-white">↻ Rescan applications</View></Focusable></View>}
    </View>
  );
}
mount(() => <App />);
