import {createContext, useContext} from "react";
import type {Abilities, ResourceDefinition} from "../api/services";

/**
 * What the session may see and do, as its permissions say: the resources
 * it may list, each with what it may ask of them, and what it may do of the
 * workload. A hint of what to offer: the estate decides every request.
 */
export type Access = {
  definitions: ResourceDefinition[];
  abilities: Abilities;
};

export const AccessContext = createContext<Access | null>(null);

/** What the session may see and do, which only an AccessProvider gives. */
export function useAccess(): Access {
  const access = useContext(AccessContext);
  if (access === null) {
    throw new Error("useAccess is used outside an AccessProvider");
  }

  return access;
}

/** What the session may do of the workload. */
export function useAbilities(): Abilities {
  return useAccess().abilities;
}

/** Whether two reads of what the session may do say the same. */
export function sameAccess(a: Access | undefined, b: Access): boolean {
  return a !== undefined && JSON.stringify(a) === JSON.stringify(b);
}
