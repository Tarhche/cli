import {describe, expect, it} from "vitest";
import {ago, bytes, canStart, canStop, lifetime, moment, portsOf, share, sizeOf, stateColor} from "./format";

describe("bytes", () => {
  it("reads a size in the largest unit it fills", () => {
    expect(bytes(0)).toBe("0 B");
    expect(bytes(512)).toBe("512 B");
    expect(bytes(1536)).toBe("1.5 KiB");
    expect(bytes(128 * 1024 * 1024)).toBe("128 MiB");
    expect(bytes(4 * 1024 ** 3)).toBe("4 GiB");
    expect(bytes(1.25 * 1024 ** 4)).toBe("1.3 TiB");
  });

  it("is nothing for what is no size", () => {
    expect(bytes(undefined)).toBe("");
    expect(bytes(null)).toBe("");
    expect(bytes(-1)).toBe("");
    expect(bytes(Number.NaN)).toBe("");
  });
});

describe("ago", () => {
  const now = new Date("2026-11-22T10:00:00Z");

  it("says how long ago a moment was, or until it", () => {
    expect(ago("2026-11-22T09:57:00Z", now)).toBe("3 minutes ago");
    expect(ago("2026-11-22T10:00:00Z", now)).toBe("now");
    expect(ago("2026-11-24T10:00:00Z", now)).toBe("in 2 days");
    expect(ago("2025-11-22T10:00:00Z", now)).toBe("last year");
  });

  it("is nothing for no moment, or the zero one", () => {
    expect(ago(undefined, now)).toBe("");
    expect(ago("not a moment", now)).toBe("");
    expect(ago("0001-01-01T00:00:00Z", now)).toBe("");
  });
});

describe("moment", () => {
  it("reads a moment on this machine, and nothing of none", () => {
    expect(moment("2026-11-22T10:00:00Z")).not.toBe("");
    expect(moment(undefined)).toBe("");
    expect(moment("nope")).toBe("");
    expect(moment("0001-01-01T00:00:00Z")).toBe("");
  });
});

describe("states", () => {
  it("colours what runs, what stopped, what failed and what is on its way", () => {
    expect(stateColor("running")).toBe("green");
    expect(stateColor("stopped")).toBe("gray");
    expect(stateColor("failed")).toBe("red");
    expect(stateColor("starting")).toBe("blue");
    expect(stateColor(undefined)).toBe("gray");
  });

  it("starts what stopped, and stops what runs", () => {
    expect(canStart("stopped")).toBe(true);
    expect(canStart("running")).toBe(false);
    expect(canStop("running")).toBe(true);
    expect(canStop("stopped")).toBe(false);
  });
});

describe("sizes", () => {
  it("reads what a VM is given", () => {
    expect(sizeOf({cpus: 1, memory: 512 * 1024 ** 2, disk: 4 * 1024 ** 3})).toBe("1 vCPU · 512 MiB · 4 GiB");
    expect(sizeOf({cpus: 2, memory: 0, disk: 0})).toBe("2 vCPUs · 0 B · 0 B");
    expect(sizeOf(undefined)).toBe("");
  });

  it("is a share of a limit, at most all of it", () => {
    expect(share(50, 200)).toBe(25);
    expect(share(300, 200)).toBe(100);
    expect(share(1, 0)).toBe(0);
    expect(share(undefined, 10)).toBe(0);
  });
});

describe("lifetime", () => {
  it("reads how long something is kept", () => {
    expect(lifetime(0)).toBe("until it is deleted");
    expect(lifetime(undefined)).toBe("until it is deleted");
    expect(lifetime(3600e9)).toBe("1 hour");
    expect(lifetime(26 * 3600e9)).toBe("1 day, 2 hours");
    expect(lifetime(90 * 60e9)).toBe("1 hour, 30 minutes");
    expect(lifetime(30e9)).toBe("30 seconds");
  });
});

describe("portsOf", () => {
  it("reads ports as docker ps does", () => {
    expect(portsOf([{container_port: 80, host_port: 8080}, {container_port: 53, host_port: 0, protocol: "udp"}])).toBe("8080->80/tcp, 53/udp");
    expect(portsOf(null)).toBe("");
  });
});
