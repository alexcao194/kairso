import { tr } from "@/lib/i18n";
import type { Permission } from "@/lib/domain/permissions";

type Slug = keyof ReturnType<typeof tr>["settingsNav"]["items"];
type GroupId = keyof ReturnType<typeof tr>["settingsNav"]["groups"];

const NAV: Array<{ group: GroupId; items: Array<{ slug: Slug; perm?: Permission }> }> = [
  { group: "people", items: [{ slug: "members" }, { slug: "invites" }, { slug: "roles" }] },
  { group: "workspace", items: [{ slug: "workspace" }, { slug: "resources" }, { slug: "labels" }] },
  { group: "ops", items: [{ slug: "attendance" }, { slug: "shifts" }, { slug: "kpi", perm: "kpi.settings" }] },
  { group: "system", items: [{ slug: "notifications" }, { slug: "integrations" }, { slug: "webhooks" }, { slug: "ai" }, { slug: "audit" }, { slug: "whats-new" }] },
];

/** Settings navigation with labels in the current language. */
export function settingsNav() {
  const t = tr().settingsNav;
  return NAV.map((g) => ({
    id: g.group,
    group: t.groups[g.group],
    items: g.items.map((i) => ({ ...i, label: t.items[i.slug] })),
  }));
}
