import {vi} from "vitest";
import type {ContextInfo, Status} from "../api/services";
import type {SessionState} from "../session/session";

/** The context a test talks to. */
export const testContext: ContextInfo = {name: "local", description: "", api: "http://localhost:8000", ingress: "http://localhost:8030", current: true};

/** A session as a test says it is: logged out unless it says otherwise. */
export function session(state: Partial<SessionState> = {}): SessionState {
  return {
    status: {context: testContext, logged_in: false, expires_at: ""} as Status,
    failure: null,
    contexts: [testContext, {...testContext, name: "default", api: "https://backend.tarhche.com", current: false}],
    refresh: vi.fn(() => Promise.resolve()),
    use: vi.fn(() => Promise.resolve()),
    ...state,
  };
}
