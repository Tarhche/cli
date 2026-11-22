import {describe, expect, it} from "vitest";
import {COMPOSE_EXAMPLE, defaultStackForm, stackFieldOf, stackRequestOf} from "./stackForm";

describe("the stack form", () => {
  it("starts from an example, in a new Docker VM or the one it was opened for", () => {
    expect(defaultStackForm()).toEqual({name: "", compose: COMPOSE_EXAMPLE, where: {fresh: true, vm: null, name: ""}});
    expect(defaultStackForm("v-1").where).toEqual({fresh: false, vm: "v-1", name: ""});
  });

  it("asks for a stack as the API takes it", () => {
    expect(stackRequestOf({name: " site ", compose: "services: {}", where: {fresh: false, vm: "v-1", name: ""}})).toEqual({name: "site", compose: "services: {}", vm_uuid: "v-1"});
  });

  it("says what is wrong in the compose file beside it", () => {
    expect(stackFieldOf("name")).toBe("name");
    expect(stackFieldOf("compose")).toBe("compose");
    expect(stackFieldOf("compose.services.web.build")).toBe("compose");
    expect(stackFieldOf("vm_uuid")).toBe("vm");
    expect(stackFieldOf("vm.network.egress")).toBe("vm");
    expect(stackFieldOf("other")).toBeUndefined();
  });
});
