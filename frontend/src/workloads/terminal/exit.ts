import type {TerminalExit} from "../../api/services";

/** How a terminal ended, as it is said at its end. */
export function exitOf(exit: TerminalExit): string {
  if (exit.error) {
    return exit.code > 0 ? `exited with ${exit.code}: ${exit.error}` : `ended: ${exit.error}`;
  }

  return exit.code === 0 ? "ended" : `exited with ${exit.code}`;
}
