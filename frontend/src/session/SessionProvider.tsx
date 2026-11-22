import {type ReactNode, useCallback, useEffect, useMemo, useRef, useState} from "react";
import {type Failure, failureOf} from "../api/failure";
import {type ContextInfo, Session, type Status} from "../api/services";
import {SignedOutContext} from "../api/signedOut";
import {endedBy, endOf, FOCUS_PAUSE, SESSION_POLL, SessionContext, type SessionEnd, type SessionState} from "./session";

type Read = {
  status: Status | undefined;
  failure: Failure | null;
  contexts: ContextInfo[];
  ended: SessionEnd | null;
};

/**
 * Reads the session the app has and the contexts it can talk to: again
 * every SESSION_POLL while there is a session, whenever the window comes
 * back to the front, and whenever a call says the session is gone, which
 * shows the login, saying why when it was told.
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

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (!signedIn) {
      return;
    }

    const timer = setInterval(() => void refresh(), SESSION_POLL);

    return () => clearInterval(timer);
  }, [signedIn, refresh]);

  useEffect(() => {
    const comeBack = () => {
      if (document.visibilityState === "visible" && Date.now() - lastRead.current >= FOCUS_PAUSE) {
        void refresh();
      }
    };

    window.addEventListener("focus", comeBack);
    document.addEventListener("visibilitychange", comeBack);

    return () => {
      window.removeEventListener("focus", comeBack);
      document.removeEventListener("visibilitychange", comeBack);
    };
  }, [refresh]);

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

  const state = useMemo<SessionState>(() => ({...read, refresh, use}), [read, refresh, use]);

  return (
    <SessionContext.Provider value={state}>
      <SignedOutContext.Provider value={signedOut}>{children}</SignedOutContext.Provider>
    </SessionContext.Provider>
  );
}
