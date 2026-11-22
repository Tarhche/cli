import {createContext, useContext} from "react";

/**
 * What to do once a call says the session is gone: the session's provider
 * reads it again, which shows the login. Nothing, outside one.
 */
export const SignedOutContext = createContext<() => void>(() => {});

/** What to do once a call says the session is gone. */
export function useSignedOut(): () => void {
  return useContext(SignedOutContext);
}
