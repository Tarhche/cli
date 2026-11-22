import {type ReactNode, useCallback, useEffect, useMemo, useRef, useState} from "react";
import {type Failure, failureOf, isCancelled} from "../api/failure";
import {type ContextInfo, Session, type Status} from "../api/services";
import {SignedOutContext} from "../api/signedOut";
import {endedBy, endOf, FOCUS_PAUSE, SESSION_POLL, SessionContext, type SessionEnd, type SessionState} from "./session";

type Read = {
  status: Status | undefined;
  failure: Failure | null;
  contexts: ContextInfo[];
  ended: SessionEnd | null;
};

/** What was read, with the permissions a session's token says now, when they changed. */
function withPermissions(read: Read, permissions: string[]): Read {
  const status = read.status;
  if (status === undefined || !status.logged_in || JSON.stringify(status.permissions ?? []) === JSON.stringify(permissions)) {
    return read;
  }

  return {...read, status: {...status, permissions}};
}

/**
 * Reads the session the app has and the contexts it can talk to, and again
 * whenever a call says the session is gone, which shows the login, saying
 * why when it was told. While there is a session, what its token says it
 * may do is read again every SESSION_POLL and whenever the window comes
 * back to the front, which asks the estate nothing but the token's renewal:
 * a role granted or withdrawn shows once the token is renewed, and a
 * session that ended or an account refused once its renewal is refused.
 */
export function SessionProvider({children}: {children: ReactNode}) {
  const [read, setRead] = useState<Read>({status: undefined, failure: null, contexts: [], ended: null});
  const rounds = useRef(0);
  const lastRead = useRef(0);
  const context = useRef<string | undefined>(undefined);
  const signedIn = read.status?.logged_in ?? false;

  const refresh = useCallback(async () => {
    const round = ++rounds.current;
    lastRead.current = Date.now();

    try {
      const [status, contexts] = await Promise.all([Session.Status(), Session.Contexts()]);
      if (round === rounds.current) {
        context.current = status.context.name;
        setRead((previous) => ({
          status,
          failure: null,
          contexts: contexts ?? [],
          ended: endOf(status) ?? (status.logged_in ? null : previous.ended),
        }));
      }
    } catch (error) {
      if (round === rounds.current) {
        setRead((previous) => ({...previous, failure: failureOf(error)}));
      }
    }
  }, []);

  const use = useCallback(
    async (name: string) => {
      await Session.Use(name);
      await refresh();
    },
    [refresh],
  );

  const signedOut = useCallback(
    (failure: Failure) => {
      const ended = endedBy(failure, context.current ?? "");
      if (ended !== null) {
        setRead((previous) => ({...previous, ended}));
      }

      void refresh();
    },
    [refresh],
  );

  const check = useCallback(async () => {
    lastRead.current = Date.now();

    try {
      const permissions = (await Session.Permissions()) ?? [];
      setRead((previous) => withPermissions(previous, permissions));
    } catch (error) {
      const failure = failureOf(error);
      if (!isCancelled(error) && failure.logged_out) {
        signedOut(failure);
      }
    }
  }, [signedOut]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!signedIn) {
      return;
    }

    const timer = setInterval(() => void check(), SESSION_POLL);

    return () => clearInterval(timer);
  }, [signedIn, check]);

  useEffect(() => {
    const comeBack = () => {
      if (document.visibilityState === "visible" && Date.now() - lastRead.current >= FOCUS_PAUSE) {
        void (signedIn ? check() : refresh());
      }
    };

    window.addEventListener("focus", comeBack);
    document.addEventListener("visibilitychange", comeBack);

    return () => {
      window.removeEventListener("focus", comeBack);
      document.removeEventListener("visibilitychange", comeBack);
    };
  }, [signedIn, check, refresh]);

  const state = useMemo<SessionState>(() => ({...read, refresh, use}), [read, refresh, use]);

  return (
    <SessionContext.Provider value={state}>
      <SignedOutContext.Provider value={signedOut}>{children}</SignedOutContext.Provider>
    </SessionContext.Provider>
  );
}
