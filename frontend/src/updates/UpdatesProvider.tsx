import {type ReactNode, useCallback, useEffect, useMemo, useState} from "react";
import {onCheckForUpdates} from "../api/events";
import {Updates} from "../api/services";
import {useQuery} from "../api/useQuery";
import {UpdateDialog} from "./UpdateDialog";
import {UpdatesContext} from "./updates";

/**
 * Says which version of tarhche the app is, and opens the dialog that looks
 * for a newer one whenever something asks: a menu of the app's, or anything
 * inside it that useUpdates' check is called from.
 */
export function UpdatesProvider({children}: {children: ReactNode}) {
  const [opened, setOpened] = useState(false);
  const version = useQuery("updates:version", readVersion);
  const check = useCallback(() => setOpened(true), []);
  const state = useMemo(() => ({version: version.data, check}), [version.data, check]);

  useEffect(() => onCheckForUpdates(check), [check]);

  return (
    <UpdatesContext.Provider value={state}>
      {children}
      <UpdateDialog opened={opened} onClose={() => setOpened(false)} />
    </UpdatesContext.Provider>
  );
}

/** The version of tarhche running. */
function readVersion(): Promise<string> {
  return Updates.Version();
}
