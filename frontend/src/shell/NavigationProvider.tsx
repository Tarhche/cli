import {type ReactNode, useCallback, useMemo, useState} from "react";
import {type History, HOME, type Navigation, NavigationContext, popped, pushed, type View} from "./navigation";

/** Keeps where the app is, starting at start. */
export function NavigationProvider({children, start = HOME}: {children: ReactNode; start?: View}) {
  const [history, setHistory] = useState<History>([start]);

  const navigate = useCallback((view: View) => setHistory((h) => pushed(h, view)), []);
  const back = useCallback(() => setHistory(popped), []);

  const navigation = useMemo<Navigation>(
    () => ({view: history[history.length - 1] ?? HOME, navigate, back}),
    [history, navigate, back],
  );

  return <NavigationContext.Provider value={navigation}>{children}</NavigationContext.Provider>;
}
