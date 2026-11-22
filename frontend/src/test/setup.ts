import "@testing-library/jest-dom/vitest";
import {cleanup} from "@testing-library/react";
import {afterEach, vi} from "vitest";

vi.mock("@wailsio/runtime", () => ({
  Events: {On: vi.fn(() => () => undefined)},
  Browser: {OpenURL: vi.fn(() => Promise.resolve())},
  Call: {ByID: vi.fn(() => Promise.resolve(null))},
  Create: {Any: (value: unknown) => value, Array: () => (value: unknown) => value, Nullable: () => (value: unknown) => value, Map: () => (value: unknown) => value, Events: {}},
  CancellablePromise: Promise,
}));

afterEach(() => {
  cleanup();
});

Object.defineProperty(window, "matchMedia", {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
});

class ResizeObserverStub {
  observe() {}
  unobserve() {}
  disconnect() {}
}

window.ResizeObserver = ResizeObserverStub;
Object.defineProperty(document, "fonts", {value: {addEventListener: () => undefined, removeEventListener: () => undefined, ready: Promise.resolve()}});
window.HTMLElement.prototype.scrollIntoView = () => undefined;
window.HTMLElement.prototype.scrollTo = () => undefined;
