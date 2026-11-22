import {createContext, useContext} from "react";
import type {Failure} from "./failure";

/** What to do once a call fails saying the session is gone. */
export type SignedOut = (failure: Failure) => void;

/**
 * What to do once a call says the session is gone, given what it said: the
 * session's provider keeps why, and reads the session again, which shows
 * the login. Nothing, outside one.
 */
export const SignedOutContext = createContext<SignedOut>(() => {});

/** What to do once a call says the session is gone. */
export function useSignedOut(): SignedOut {
  return useContext(SignedOutContext);
}
