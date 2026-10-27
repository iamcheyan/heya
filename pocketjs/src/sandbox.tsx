import { render } from "solid-js/web";
import { createSignal, For, onCleanup, onMount } from "solid-js";
import "./style.css";

type App = { id: string; name: string; category: string; icon: string; accent: string; subtitle: string };
const mockApps: App[] = [
  { id: "books", name: "Books", category: "READ", icon: "▤", accent: "#d3a86e", subtitle: "Continue reading" },
  { id: "music", name: "Music", category: "LISTEN", icon: "♫", accent: "#83a8dc", subtitle: "Now playing" },
  { id: "library", name: "Library", category: "PLAY", icon: "✦", accent: "#e5bd61", subtitle: "Your collection" },
  { id: "notes", name: "Notes", category: "CREATE", icon: "✎", accent: "#89b69d", subtitle: "A quiet workspace" },
  { id: "settings", name: "Settings", category: "SYSTEM", icon: "◉", accent: "#9a9ab8", subtitle: "Make it yours" },
];
function App() {
  const [apps] = createSignal(mockApps);
  const [index, setIndex] = createSignal(new URLSearchParams(location.search).get("focus") ? Math.max(0, mockApps.findIndex((app) => app.id === new URLSearchParams(location.search).get("focus"))) : 1);
  const [favorite, setFavorite] = createSignal(false);
  const [menuOpen, setMenuOpen] = createSignal(false);
  const [clock, setClock] = createSignal(new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" }));
  const [message, setMessage] = createSignal("A quiet place for the things you use every day");
  let root!: HTMLDivElement;
  let timer = 0;
  const move = (delta: number) => setIndex((current) => (current + delta + apps().length) % apps().length);
  const active = () => apps()[index()];
  const launch = () => setMessage(`${active().name} is ready to launch · prototype keeps the existing launcher untouched`);
  const toggleFavorite = () => { const next = !favorite(); setFavorite(next); setMessage(`${active().name} ${next ? "added to" : "removed from"} favorites`); };
  const onKey = (event: KeyboardEvent) => {
    const key = event.key.toLowerCase();
    if (["arrowleft", "arrowright", "enter", "escape", "x", "y", "a", "b"].includes(key)) event.preventDefault();
    if (event.key === "ArrowLeft") move(-1);
    else if (event.key === "ArrowRight") move(1);
    else if (event.key === "Enter" || key === "a") launch();
    else if (key === "y") toggleFavorite();
    else if (key === "x") setMenuOpen((value) => !value);
    else if (event.key === "Escape" || key === "b") setMenuOpen(false);
  };
  onMount(() => {
    root.focus();
    timer = window.setInterval(() => setClock(new Date().toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" })), 30_000);
    window.addEventListener("keydown", onKey);
    onCleanup(() => window.removeEventListener("keydown", onKey));
  });
  onCleanup(() => window.clearInterval(timer));
  const offset = (i: number) => { let d = i - index(); const n = apps().length; if (d > n / 2) d -= n; if (d < -n / 2) d += n; return d; };
  return (
    <div class="shell" ref={root} tabIndex={0}>
      <header class="status"><div class="brand"><span class="brand-mark">h</span><span>heya<span class="brand-dot">.</span></span></div><div class="status-right"><span class="connectivity">◉ <span>Wi-Fi</span></span><span class="battery-wrap">▰ <span>—</span></span><span class="clock">{clock()}</span></div></header>
      <main class="home"><div class="eyebrow"><span>YOUR POCKET, IN FLOW</span><span class="eyebrow-rule"></span><span class="count">{String(index() + 1).padStart(2, "0")} <em>/ {String(apps().length).padStart(2, "0")}</em></span></div>
        <div class="showcase"><div class="ambient" style={{ "--accent": active().accent }}></div>
          <For each={apps()}>{(app, i) => { const d = () => offset(i()); return <button class="app-card" classList={{ selected: d() === 0, distant: Math.abs(d()) > 1 }} style={{ "--position": d(), "--accent": app.accent }} onClick={() => setIndex(i())}>
            <span class="card-top"><span>{app.category}</span><span class="card-spark">✳</span></span><span class="app-icon"><span class="icon-glyph">{app.icon}</span><span class="icon-orbit"></span></span><span class="card-bottom"><span class="app-name">{app.name}</span><span class="card-arrow">↗</span></span><span class="card-glow"></span>
          </button>; }}</For>
          <button class="arrow arrow-left" onClick={() => move(-1)}>←</button><button class="arrow arrow-right" onClick={() => move(1)}>→</button>
        </div><div class="app-caption"><span class="caption-bullet" style={{ "--accent": active().accent }}></span><span>{active().subtitle}</span><span class="caption-separator">/</span><span class="caption-muted">{active().category.toLowerCase()} at your pace</span></div>
      </main>
      <footer class="dock"><div class="dock-message"><span class="message-spark">✳</span><span>{message()}</span></div><nav class="actions" aria-label="Controls"><button onClick={launch}><kbd>A</kbd><span>Open</span></button><button onClick={() => setMenuOpen(false)}><kbd>B</kbd><span>Back</span></button><button onClick={() => setMenuOpen((value) => !value)}><kbd>X</kbd><span>Quick menu</span></button><button onClick={toggleFavorite}><kbd>Y</kbd><span>{favorite() ? "Saved" : "Favorite"}</span></button></nav></footer>
      {menuOpen() && <div class="menu-scrim" onClick={() => setMenuOpen(false)}><section class="quick-menu" onClick={(event) => event.stopPropagation()}><div class="menu-heading"><span>QUICK MENU</span><button onClick={() => setMenuOpen(false)}>×</button></div><button class="menu-item" onClick={toggleFavorite}><span>☆</span><span>{favorite() ? "Remove favorite" : "Add to favorites"}</span><span>Y</span></button><button class="menu-item" onClick={() => { setMessage("Application rescan requested · Registry integration phase"); setMenuOpen(false); }}><span>↻</span><span>Rescan applications</span><span>↵</span></button></section></div>}
      <div class="page-index"><span>HOME</span><span class="index-line"></span><span>01</span></div>
    </div>
  );
}
export default App;

if (typeof document !== "undefined") {
  const rootEl = document.getElementById("root");
  if (rootEl) {
    const mountApp = () => render(() => App(), rootEl);
    if (document.readyState === "loading") {
      document.addEventListener("DOMContentLoaded", mountApp, { once: true });
    } else {
      mountApp();
    }
  }
}
