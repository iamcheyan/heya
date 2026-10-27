import { resolve } from "node:path";

const root = resolve(process.env.HEYA_ROOT ?? process.cwd());
const apps = resolve(process.env.HEYA_APPS ?? `${root}/examples/apps`);
const go = process.env.HEYA_GO ?? "go";
const scanner = process.env.HEYA_REGISTRY_SCANNER;

export async function registryResponse(): Promise<Response> {
  const command = scanner ? [scanner, "-dir", apps] : [go, "run", "./pocketjs/tools/dump-apps", "-dir", apps];
  const result = await Bun.spawn(command, {
    cwd: root,
    stdout: "pipe",
    stderr: "pipe",
  });
  const [stdout, stderr, status] = await Promise.all([
    new Response(result.stdout).text(),
    new Response(result.stderr).text(),
    result.exited,
  ]);
  if (status !== 0) {
    return Response.json({ error: stderr || "Registry scan failed" }, { status: 500 });
  }
  try {
    const data: unknown = JSON.parse(stdout);
    if (!Array.isArray(data)) throw new Error("Registry output must be an array");
    return Response.json(data, { headers: { "cache-control": "no-store" } });
  } catch (error) {
    return Response.json({ error: `Invalid Registry JSON: ${String(error)}` }, { status: 500 });
  }
}
