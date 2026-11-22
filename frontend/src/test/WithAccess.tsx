import type {ReactNode} from "react";
import {type Access, AccessContext} from "../access/access";
import {access as allowing} from "./access";

/** Renders what it is given for a session that may see and do what access says: everything of the workload unless it says otherwise. */
export function WithAccess({access = allowing(), children}: {access?: Access; children: ReactNode}) {
  return <AccessContext.Provider value={access}>{children}</AccessContext.Provider>;
}
