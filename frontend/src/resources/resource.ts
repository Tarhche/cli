import type {ResourceAction, ResourceDefinition, ResourceFilter, ResourceLink, Routes, Row} from "../api/services";

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
 * audience's own, or the other's when only it has one of that name, when
 * the session may ask it; those asked of one thing when keyed, those asked
 * of the resource otherwise.
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

  return [...chosen.values()].filter((action) => action.keyed === keyed && action.allowed);
}

/** The links an audience of a resource is offered, in the order they are offered. */
export function linksFor(definition: ResourceDefinition, audience: Audience): ResourceLink[] {
  return (definition.links ?? []).filter((link) => (link.audiences ?? []).includes(audience));
}

/** Where the links offered lead for a row: those it has somewhere to go for, each with where. */
export function rowLinks(links: ResourceLink[], row: Row): {link: ResourceLink; to: string}[] {
  return links.flatMap((link) => {
    const to = row.links?.[link.name];

    return to ? [{link, to}] : [];
  });
}

/** The actions offered of one thing that are offered for a row. */
export function rowActions(actions: ResourceAction[], row: Row): ResourceAction[] {
  return actions.filter((action) => (row.actions ?? []).includes(action.name));
}

/** The audience a page shows: the one chosen while it may still be listed, else the first that may. */
export function audienceShown(audiences: Audience[], chosen: Audience): Audience {
  return audiences.includes(chosen) ? chosen : (audiences[0] ?? chosen);
}

/** The filters the listing of an audience of a resource takes. */
export function filtersFor(definition: ResourceDefinition, audience: Audience): ResourceFilter[] {
  return (definition.filters ?? []).filter((filter) => (filter.audiences ?? []).length === 0 || filter.audiences.includes(audience));
}

/** What a row is called where it is named: the first thing it says, or its key. */
export function nameOf(key: string[], cells: string[]): string {
  return cells.find((cell) => cell.trim() !== "") ?? key.join("/");
}

/** Filters as a listing is narrowed by them: those that say nothing left out. */
export function said(filters: Record<string, string>): Record<string, string> {
  return Object.fromEntries(Object.entries(filters).filter(([, value]) => value.trim() !== ""));
}
