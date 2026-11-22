import {createContext, useContext} from "react";

/** The version of tarhche the app is, and looking for a newer one. */
export type UpdatesState = {
  /** The version running, once it was read. */
  version: string | undefined;

  /** Opens the dialog that looks for a newer release and upgrades to it. */
  check: () => void;
};

/** What an UpdatesProvider gives: outside one, no version and nothing to open. */
export const UpdatesContext = createContext<UpdatesState>({version: undefined, check: () => undefined});

/** The version of tarhche the app is, and looking for a newer one. */
export function useUpdates(): UpdatesState {
  return useContext(UpdatesContext);
}
