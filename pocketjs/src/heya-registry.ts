export type HeYaApp = { id: string; name: string; category?: string; icon?: string; accent?: string; subtitle?: string; entrypoint?: string };

export type PocketCard = {
  id: string;
  name: string;
  category: string;
  icon: string;
  accent: string;
  subtitle: string;
  entrypoint: string;
};

/** Project the serialized output of heya's Registry into PocketJS card data. */
export function toPocketCards(apps: HeYaApp[]): PocketCard[] {
  return apps.map((app) => ({
    id: app.id,
    name: app.name,
    category: app.category || "APPS",
    icon: app.icon || "✦",
    accent: app.accent || "#c3a873",
    subtitle: app.subtitle || "Ready when you are",
    entrypoint: app.entrypoint || "",
  }));
}
