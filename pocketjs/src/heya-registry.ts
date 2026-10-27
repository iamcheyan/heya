export type HeYaApp = { id?: string; name: string; category?: string; icon?: string; accent?: string; description?: string; entrypoint?: string; exec?: string };

export type PocketCard = {
  id: string;
  name: string;
  category: string;
  icon: string;
  accent: string;
  subtitle: string;
  entrypoint: string;
};

/** Project heya manifest data into the view model without moving launch logic.
 * Production should consume Registry output serialized across an explicit bridge;
 * this browser prototype uses mock records because Go packages cannot be imported
 * into PocketJS's JavaScript guest.
 */
export function toPocketCards(apps: HeYaApp[]): PocketCard[] {
  return apps.map((app) => ({
    id: app.id ?? app.name.toLowerCase().replaceAll(" ", "-"),
    name: app.name,
    category: app.category || "APPS",
    icon: app.icon || "✦",
    accent: app.accent || "#c3a873",
    subtitle: app.description || "Ready when you are",
    entrypoint: app.entrypoint || app.exec || "",
  }));
}
