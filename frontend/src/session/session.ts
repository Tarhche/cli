import {createContext, useContext} from "react";
import type {Failure} from "../api/failure";
import type {ContextInfo, Status} from "../api/services";

/**
 * How often the session is read again while there is one: its access token,
 * and what it says the session may do, is renewed every few minutes.
 */
export const SESSION_POLL = 60_000;

/** How long after a read the window coming back to the front reads it again. */
export const FOCUS_PAUSE = 10_000;

/** Why the session the app had in a context ended, as it was told. */
export type SessionEnd = {
  /** The context whose session it was. */
  context: string;

  /** Whether the estate refuses the account itself, rather than the session having run out. */
  blocked: boolean;

  /** Why, as the estate said it, when it said. */
  reason: string;
};

/** The session the app has, in the context it talks to, and its contexts. */
export type SessionState = {
  /** The session, once it was read. */
  status: Status | undefined;

  /** Why the session or the contexts could not be read. */
  failure: Failure | null;

  contexts: ContextInfo[];

  /** Why the last session ended, when that was told and nobody signed in since. */
  ended: SessionEnd | null;

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

/** Why a session ended, as reading it said, when it said. */
export function endOf(status: Status): SessionEnd | null {
  if (!status.ended && !status.blocked) {
    return null;
  }

  return {context: status.context.name, blocked: status.blocked ?? false, reason: status.reason ?? ""};
}

/** Why the session of context ended, as a call that failed said, when it said. */
export function endedBy(failure: Failure, context: string): SessionEnd | null {
  if (!failure.ended && !failure.blocked) {
    return null;
  }

  return {context, blocked: failure.blocked ?? false, reason: failure.reason ?? ""};
}

/** Why the session of context ended, when that is what ended says. */
export function endOfContext(ended: SessionEnd | null, context: string | undefined): SessionEnd | null {
  return ended !== null && ended.context === context ? ended : null;
}
