import {describe, expect, it} from "vitest";
import {failureOf, formFailure, isCancelled} from "./failure";

describe("failureOf", () => {
  it("is the failure a call's error carries", () => {
    const error = new Error("name: required", {cause: {message: "name: required", status: 400, fields: {name: "required"}}});

    expect(failureOf(error)).toEqual({message: "name: required", status: 400, fields: {name: "required"}});
  });

  it("is an error's message when it carries none", () => {
    expect(failureOf(new Error("it broke"))).toEqual({message: "it broke"});
    expect(failureOf(new Error("it broke", {cause: "not a failure"}))).toEqual({message: "it broke"});
  });

  it("is anything else as it is written", () => {
    expect(failureOf("plain")).toEqual({message: "plain"});
    expect(failureOf(null)).toEqual({message: "null"});
    expect(failureOf({nothing: true})).toEqual({message: "[object Object]"});
  });
});

describe("isCancelled", () => {
  it("knows a cancelled call", () => {
    expect(isCancelled(Object.assign(new Error("x"), {name: "CancelError"}))).toBe(true);
    expect(isCancelled(Object.assign(new Error("x"), {name: "CancelledRejectionError"}))).toBe(true);
  });

  it("knows anything else is not one", () => {
    expect(isCancelled(new Error("x"))).toBe(false);
    expect(isCancelled("CancelError")).toBe(false);
  });
});

describe("formFailure", () => {
  const fieldOf = (field: string) => ({name: "name", "resources.memory": "memory", "resources.disk": "memory"})[field];

  it("says nothing of no failure", () => {
    expect(formFailure(null, fieldOf)).toEqual({fields: {}, messages: []});
  });

  it("says a failure that refused no field above the form", () => {
    expect(formFailure({message: "it is not running"}, fieldOf)).toEqual({fields: {}, messages: ["it is not running"]});
  });

  it("says what was refused of a field beside it, the first said first, and the rest above", () => {
    const laidOut = formFailure(
      {message: "...", fields: {name: "required", "resources.memory": "too small", "resources.disk": "too small too", lifetime: "too long"}},
      fieldOf,
    );

    expect(laidOut).toEqual({fields: {name: "required", memory: "too small"}, messages: ["lifetime: too long"]});
  });
});
