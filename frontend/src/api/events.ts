import {Events} from "@wailsio/runtime";
import type {ForwardStopped, TerminalExit, UpdateProgress} from "./services";

/** Stops listening. */
export type Unsubscribe = () => void;

/**
 * Listens to what the terminal id says, as the bytes it said, encoded in
 * base64.
 */
export function onTerminalOutput(id: string, listener: (data: string) => void): Unsubscribe {
  return Events.On("terminal:output", (event) => {
    if (event.data.id === id) {
      listener(event.data.data);
    }
  });
}

/** Listens for the terminal id to end. */
export function onTerminalExit(id: string, listener: (exit: TerminalExit) => void): Unsubscribe {
  return Events.On("terminal:exit", (event) => {
    if (event.data.id === id) {
      listener(event.data);
    }
  });
}

/** Listens for port forwards that stop by themselves. */
export function onForwardStopped(listener: (stopped: ForwardStopped) => void): Unsubscribe {
  return Events.On("forward:stopped", (event) => listener(event.data));
}

/** Listens for the app's menus asking to look for a newer release. */
export function onCheckForUpdates(listener: () => void): Unsubscribe {
  return Events.On("updates:check", () => listener());
}

/** Listens to how much of an upgrade's download has arrived. */
export function onUpdateProgress(listener: (progress: UpdateProgress) => void): Unsubscribe {
  return Events.On("updates:progress", (event) => listener(event.data));
}
