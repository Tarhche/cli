import {describe, expect, it} from "vitest";
import {deviceTypeName, type LoginSession, seenAs, sessionDevice, sessionKind, sessionName, sessionSystem, sessionTitle} from "./sessions";

/** A session signed in to from a browser, but for what a test says. */
function session(overrides: Partial<LoginSession> = {}): LoginSession {
  return {
    uuid: "s-1",
    current: false,
    device: {browser: "Firefox", browser_version: "128.0.3", os: "macOS", os_version: "15.1", type: "desktop", model: "Mac"},
    created_at: "2026-11-01T10:00:00Z",
    last_activity: {at: "2026-11-22T10:00:00Z"},
    expires_at: "2026-12-22T10:00:00Z",
    ...overrides,
  };
}

describe("sessionKind", () => {
  it("is an application's, a shadow one, or the kind of device it runs on", () => {
    expect(sessionKind(session({client: {id: "c-1", name: "Tarhche CLI"}}))).toBe("application");
    expect(sessionKind(session({impersonated_user: {uuid: "u-2", name: "Sam Rivera"}}))).toBe("shadow");
    expect(sessionKind(session())).toBe("desktop");
    expect(sessionKind(session({device: {type: "mobile"}}))).toBe("mobile");
    expect(sessionKind(session({device: {type: "tablet"}}))).toBe("tablet");
    expect(sessionKind(session({device: {type: "bot"}}))).toBe("bot");
    expect(sessionKind(session({device: {type: "toaster"}}))).toBe("unknown");
    expect(sessionKind(session({device: {}}))).toBe("unknown");
  });
});

describe("sessionName", () => {
  it("is the application's name, or the browser by its major version", () => {
    expect(sessionName(session({client: {id: "c-1", name: "Tarhche CLI"}}))).toBe("Tarhche CLI");
    expect(sessionName(session({client: {id: "c-1"}}))).toBe("An application");
    expect(sessionName(session())).toBe("Firefox 128");
    expect(sessionName(session({device: {browser: "Safari"}}))).toBe("Safari");
    expect(sessionName(session({device: {}}))).toBe("Unknown device");
  });
});

describe("sessionDevice", () => {
  it("is the device's model, its system and the kind it is", () => {
    expect(sessionSystem(session())).toBe("macOS 15.1");
    expect(deviceTypeName(session({device: {type: "mobile"}}))).toBe("Phone");
    expect(deviceTypeName(session({device: {}}))).toBeUndefined();
    expect(sessionDevice(session())).toBe("Mac · macOS 15.1 · Desktop");
    expect(sessionDevice(session({device: {}}))).toBe("");
  });
});

describe("sessionTitle", () => {
  it("is its name, with the device it runs on when that is known", () => {
    expect(sessionTitle(session())).toBe("Firefox 128 (Mac · macOS 15.1 · Desktop)");
    expect(sessionTitle(session({device: {browser: "Safari"}}))).toBe("Safari");
  });
});

describe("seenAs", () => {
  it("is who a shadow session is seen as, by the name they go by", () => {
    expect(seenAs(session())).toBeUndefined();
    expect(seenAs(session({impersonated_user: {uuid: "u-2", name: "Sam Rivera", username: "sam"}}))).toBe("Sam Rivera");
    expect(seenAs(session({impersonated_user: {uuid: "u-2", username: "sam"}}))).toBe("sam");
    expect(seenAs(session({impersonated_user: {uuid: "u-2"}}))).toBe("u-2");
  });
});
