import {Events} from "@wailsio/runtime";
import type {ForwardStopped, TerminalExit} from "./services";

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
