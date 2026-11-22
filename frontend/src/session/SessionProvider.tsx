import {type ReactNode, useCallback, useEffect, useMemo, useRef, useState} from "react";
import {type Failure, failureOf} from "../api/failure";
import {type ContextInfo, Session, type Status} from "../api/services";
import {SignedOutContext} from "../api/signedOut";
import {SessionContext, type SessionState} from "./session";

type Read = {
  status: Status | undefined;
  failure: Failure | null;
  contexts: ContextInfo[];
};

/**
 * Reads the session the app has and the contexts it can talk to, and reads
 * them again whenever a call says the session is gone, which shows the
 * login.
 */
export function SessionProvider({children}: {children: ReactNode}) {
  const [read, setRead] = useState<Read>({status: undefined, failure: null, contexts: []});
  const rounds = useRef(0);

  const refresh = useCallback(async () => {
    const round = ++rounds.current;

    try {
      const [status, contexts] = await Promise.all([Session.Status(), Session.Contexts()]);
      if (round === rounds.current) {
        setRead({status, failure: null, contexts: contexts ?? []});
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

  const signedOut = useCallback(() => {
    void refresh();
  }, [refresh]);

  const state = useMemo<SessionState>(() => ({...read, refresh, use}), [read, refresh, use]);

  return (
    <SessionContext.Provider value={state}>
      <SignedOutContext.Provider value={signedOut}>{children}</SignedOutContext.Provider>
    </SessionContext.Provider>
  );
}
