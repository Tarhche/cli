import {createContext, useContext} from "react";
import type {Failure} from "../api/failure";
import type {ContextInfo, Status} from "../api/services";

/** The session the app has, in the context it talks to, and its contexts. */
export type SessionState = {
  /** The session, once it was read. */
  status: Status | undefined;

  /** Why the session or the contexts could not be read. */
  failure: Failure | null;

  contexts: ContextInfo[];

  /** Reads the session and the contexts again. */
  refresh: () => Promise<void>;

  /** Talks to the context of that name from then on. */
  use: (name: string) => Promise<void>;
};

export const SessionContext = createContext<SessionState | null>(null);

/** The session the app has, which only a SessionProvider gives. */
export function useSession(): SessionState {
  const session = useContext(SessionContext);
  if (session === null) {
    throw new Error("useSession is used outside a SessionProvider");
  }

  return session;
}
