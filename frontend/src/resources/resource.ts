import type {ResourceAction, ResourceDefinition, ResourceFilter, Routes} from "../api/services";

/** Whose things a page shows: one's own, or everybody's. */
export type Audience = "mine" | "everyone";

/** What each audience is called on a page. */
export const AUDIENCE_LABELS: Record<Audience, string> = {
  mine: "Mine",
  everyone: "Everybody's",
};

/** The audiences a resource is listed to, one's own first. */
export function audiencesOf(definition: ResourceDefinition): Audience[] {
  return (definition.audiences ?? []).filter((audience): audience is Audience => audience === "mine" || audience === "everyone");
}

/** What an audience of a resource can be asked. */
export function routesFor(definition: ResourceDefinition, audience: Audience): Routes {
  return audience === "everyone" ? definition.everyone : definition.mine;
}

/**
 * The actions an audience of a resource is offered, one of each name: the
 * audience's own, or the other's when only it has one of that name; those
 * asked of one thing when keyed, those asked of the resource otherwise.
 */
export function actionsFor(definition: ResourceDefinition, audience: Audience, keyed: boolean): ResourceAction[] {
  const mine = audience === "mine";
  const chosen = new Map<string, ResourceAction>();

  for (const action of definition.actions ?? []) {
    const was = chosen.get(action.name);
    if (was === undefined || (was.mine !== mine && action.mine === mine)) {
      chosen.set(action.name, action);
    }
  }

  return [...chosen.values()].filter((action) => action.keyed === keyed);
}

/** The filters the listing of an audience of a resource takes. */
export function filtersFor(definition: ResourceDefinition, audience: Audience): ResourceFilter[] {
  return (definition.filters ?? []).filter((filter) => (filter.audiences ?? []).length === 0 || filter.audiences.includes(audience));
}

/** What a row is called where it is named: its key, or its first cell. */
export function nameOf(key: string[], cells: string[]): string {
  return key.length > 0 ? key.join("/") : (cells[0] ?? "");
}

/** Something as JSON, indented to be read and changed. */
export function pretty(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2);
}

/** Filters as a listing is narrowed by them: those that say nothing left out. */
export function said(filters: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(filters).filter(([, value]) => value.trim() !== ""));
}
