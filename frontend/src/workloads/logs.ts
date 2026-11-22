import type {LogLine} from "./types";

/** A line of a log, as it is shown: when, from where, and what. */
export function lineOf(line: LogLine): string {
  const at = new Date(line.at);
  const when = Number.isNaN(at.getTime()) ? "" : at.toLocaleTimeString();
  const from = line.source || line.stream || "";

  return [when, from && `[${from}]`, line.line].filter(Boolean).join(" ");
}
