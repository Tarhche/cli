import {type ReactNode, useCallback, useMemo, useState} from "react";
import {type History, HOME, type Navigation, NavigationContext, popped, pushed, type View} from "./navigation";

/** Keeps where the app is, starting at home, which back comes back to when there is nothing before. */
export function NavigationProvider({children, home = HOME}: {children: ReactNode; home?: View}) {
  const [history, setHistory] = useState<History>([home]);

  const navigate = useCallback((view: View) => setHistory((h) => pushed(h, view)), []);
  const back = useCallback(() => setHistory((h) => popped(h, home)), [home]);

  const navigation = useMemo<Navigation>(
    () => ({view: history[history.length - 1] ?? home, home, navigate, back}),
    [history, home, navigate, back],
  );

  return <NavigationContext.Provider value={navigation}>{children}</NavigationContext.Provider>;
}
