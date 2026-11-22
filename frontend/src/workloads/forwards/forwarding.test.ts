import {describe, expect, it} from "vitest";
import type {Forward} from "../../api/services";
import {reaches, urlOf} from "./forwarding";

const forward = {id: "1", context: "default", target: {vm: "v-1", container: "web"}, port: 80, address: "127.0.0.1:8080", started_at: ""} as Forward;

describe("forwarding", () => {
  it("knows what a forward reaches into", () => {
    expect(reaches(forward, {vm: "v-1", container: "web"})).toBe(true);
    expect(reaches(forward, {vm: "v-1"})).toBe(false);
    expect(reaches({...forward, target: {vm: "v-1"}} as Forward, {vm: "v-1", container: ""})).toBe(true);
  });

  it("is reached at a URL on this machine", () => {
    expect(urlOf(forward)).toBe("http://127.0.0.1:8080");
  });
});
