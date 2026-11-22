import type {ReactNode} from "react";
import {SessionContext, type SessionState} from "../session/session";

/** Renders what it is given inside a session as a test says it is. */
export function WithSession({state, children}: {state: SessionState; children: ReactNode}) {
  return <SessionContext.Provider value={state}>{children}</SessionContext.Provider>;
}
