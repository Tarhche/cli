import {createContext, useContext} from "react";
import type {Access} from "../access/access";
import type {Abilities, KindAbilities, ResourceDefinition} from "../api/services";
import {forwardKindsOf} from "../workloads/forwards/forwarding";

/** A page of the workload, which the navbar offers. */
export type WorkloadPage =
  | "containers"
  | "vms"
  | "images"
  | "volumes"
  | "networks"
  | "stacks"
  | "snapshots"
  | "forwards";

/** Where the app is: a page, and what it shows. */
export type View =
  | {page: WorkloadPage}
  | {page: "vm"; uuid: string}
  | {page: "container"; vm: string; id: string}
  | {page: "create-vm"; snapshot?: string}
  | {page: "run-container"; vm?: string}
  | {page: "deploy-stack"; vm?: string}
  | {page: "resource"; name: string};

/** The pages of the workload, in the order the navbar offers them. */
export const WORKLOAD_PAGES: {page: WorkloadPage; label: string}[] = [
  {page: "containers", label: "Containers"},
  {page: "vms", label: "VMs"},
  {page: "images", label: "Images"},
  {page: "volumes", label: "Volumes"},
  {page: "networks", label: "Networks"},
  {page: "stacks", label: "Stacks"},
  {page: "snapshots", label: "Snapshots"},
  {page: "forwards", label: "Port forwards"},
];

/** Where the app starts, when the session may see it. */
export const HOME: View = {page: "containers"};

/** Whether the session may see a page of the workload: its listing, or something to forward a port of. */
function workloadPageAllowed(page: WorkloadPage, abilities: Abilities): boolean {
  if (page === "forwards") {
    return forwardKindsOf(abilities).length > 0;
  }

  const kinds: Record<Exclude<WorkloadPage, "forwards">, KindAbilities> = abilities;

  return kinds[page].list;
}

/** The pages of the workload the session may see, in the order the navbar offers them. */
export function workloadPagesOf(abilities: Abilities): {page: WorkloadPage; label: string}[] {
  return WORKLOAD_PAGES.filter(({page}) => workloadPageAllowed(page, abilities));
}

/** Whether the session may see a view: a page it may list, one it may read, or a form it may send. */
export function allows(access: Access, view: View): boolean {
  const {abilities, definitions} = access;

  switch (view.page) {
    case "vm":
      return abilities.vms.show;
    case "create-vm":
      return abilities.vms.create;
    case "container":
      return abilities.containers.show;
    case "run-container":
      return abilities.containers.create;
    case "deploy-stack":
      return abilities.stacks.create;
    case "resource":
      return definitions.some((definition) => definition.name === view.name && (definition.audiences ?? []).length > 0);
    default:
      return workloadPageAllowed(view.page, abilities);
  }
}

/** Where the app starts for a session: the first page the navbar offers it, nowhere when it offers none. */
export function homeOf(access: Access): View | null {
  const workload = workloadPagesOf(access.abilities)[0];
  if (workload !== undefined) {
    return {page: workload.page};
  }

  const resource = sectionsOf(access.definitions)[0]?.definitions[0];

  return resource === undefined ? null : {page: "resource", name: resource.name};
}

/** A section of the navbar, and the resources it lists. */
export type Section = {
  title: string;
  definitions: ResourceDefinition[];
};

/**
 * The navbar's sections of resources: one for each section the definitions
 * name, in the order they are first named, of every resource that is
 * listed to anybody.
 */
export function sectionsOf(definitions: ResourceDefinition[]): Section[] {
  const sections: Section[] = [];

  for (const definition of definitions) {
    if ((definition.audiences ?? []).length === 0) {
      continue;
    }

    const title = definition.section || "Other";
    let section = sections.find((s) => s.title === title);
    if (section === undefined) {
      section = {title, definitions: []};
      sections.push(section);
    }

    section.definitions.push(definition);
  }

  return sections;
}

/** The page of the navbar a view is under: a detail is under its listing. */
export function pageOf(view: View): string {
  switch (view.page) {
    case "vm":
    case "create-vm":
      return "vms";
    case "container":
    case "run-container":
      return "containers";
    case "deploy-stack":
      return "stacks";
    case "resource":
      return `resource:${view.name}`;
    default:
      return view.page;
  }
}

/** Where the app is, and how to go elsewhere. */
export type Navigation = {
  view: View;

  /** Where it starts, and goes back to when there is nothing before. */
  home: View;

  /** Goes to a view, which back comes back from. */
  navigate: (view: View) => void;

  /** Goes back to the view before, or home. */
  back: () => void;
};

export const NavigationContext = createContext<Navigation | null>(null);

/** Where the app is, which only a NavigationProvider gives. */
export function useNavigation(): Navigation {
  const navigation = useContext(NavigationContext);
  if (navigation === null) {
    throw new Error("useNavigation is used outside a NavigationProvider");
  }

  return navigation;
}

/** The views gone through, the current one last; what back pops. */
export type History = View[];

/** The history once view is gone to: the same view is not gone to twice. */
export function pushed(history: History, view: View): History {
  const current = history[history.length - 1];
  if (current !== undefined && JSON.stringify(current) === JSON.stringify(view)) {
    return history;
  }

  return [...history, view];
}

/** The history once back is asked for: home once there is nothing before. */
export function popped(history: History, home: View = HOME): History {
  if (history.length <= 1) {
    return [home];
  }

  return history.slice(0, -1);
}
