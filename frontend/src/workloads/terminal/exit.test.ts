import {describe, expect, it} from "vitest";
import type {TerminalExit} from "../../api/services";
import {exitOf} from "./exit";

const exit = (code: number, error = ""): TerminalExit => ({id: "t", code, error}) as TerminalExit;

describe("exitOf", () => {
  it("says how a terminal ended", () => {
    expect(exitOf(exit(0))).toBe("ended");
    expect(exitOf(exit(3))).toBe("exited with 3");
    expect(exitOf(exit(3, "exit status 3"))).toBe("exited with 3: exit status 3");
    expect(exitOf(exit(-1, "the connection was lost"))).toBe("ended: the connection was lost");
  });
});
