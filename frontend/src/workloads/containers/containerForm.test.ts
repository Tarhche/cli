import {describe, expect, it} from "vitest";
import {containerFieldOf, containerRequestOf, defaultContainerForm, envOf, splitCommand, vmChoiceOf} from "./containerForm";

describe("splitCommand", () => {
  it("splits a command line as a shell does", () => {
    expect(splitCommand("")).toEqual([]);
    expect(splitCommand("  sleep   60 ")).toEqual(["sleep", "60"]);
    expect(splitCommand(`sh -c "echo hello world"`)).toEqual(["sh", "-c", "echo hello world"]);
    expect(splitCommand(`echo 'it''s' ""`)).toEqual(["echo", "its", ""]);
    expect(splitCommand(String.raw`echo a\ b \"c\"`)).toEqual(["echo", "a b", `"c"`]);
    expect(splitCommand(String.raw`echo 'a\b'`)).toEqual(["echo", String.raw`a\b`]);
  });
});

describe("envOf", () => {
  it("is a KEY=value a line, blank lines aside", () => {
    expect(envOf("A=1\n\n  B=two words \n")).toEqual(["A=1", "B=two words"]);
  });
});

describe("vmChoiceOf", () => {
  it("names a Docker VM of one's own, or describes a new one", () => {
    expect(vmChoiceOf({fresh: false, vm: "v-1", name: ""})).toEqual({vm_uuid: "v-1"});
    expect(vmChoiceOf({fresh: false, vm: null, name: ""})).toEqual({vm_uuid: ""});
    expect(vmChoiceOf({fresh: true, vm: "v-1", name: " docker "})).toEqual({vm: {name: "docker"}});
    expect(vmChoiceOf({fresh: true, vm: null, name: " "})).toEqual({});
  });
});

describe("the container form", () => {
  it("starts in a new Docker VM, or in the one it was opened for", () => {
    expect(defaultContainerForm().where).toEqual({fresh: true, vm: null, name: ""});
    expect(defaultContainerForm("v-1").where).toEqual({fresh: false, vm: "v-1", name: ""});
  });

  it("asks for a container as the API takes it", () => {
    const request = containerRequestOf({
      ...defaultContainerForm("v-1"),
      image: " nginx ",
      name: "web",
      command: "nginx -g 'daemon off;'",
      env: "A=b",
      ports: [
        {container: 80, host: 8080, protocol: "tcp"},
        {container: "", host: 1, protocol: "tcp"},
        {container: 53, host: "", protocol: "udp"},
      ],
      restartPolicy: "always",
      cpus: 0.5,
      memoryMiB: 64,
    });

    expect(request).toEqual({
      vm_uuid: "v-1",
      image: "nginx",
      name: "web",
      command: ["nginx", "-g", "daemon off;"],
      env: ["A=b"],
      ports: [
        {container_port: 80, host_port: 8080, protocol: "tcp"},
        {container_port: 53, host_port: 0, protocol: "udp"},
      ],
      restart_policy: "always",
      cpus: 0.5,
      memory: 64 * 1024 * 1024,
    });
  });

  it("says what the API refused beside the field it is of", () => {
    expect(containerFieldOf("vm")).toBe("vm");
    expect(containerFieldOf("vm.name")).toBe("vm");
    expect(containerFieldOf("ports.0.host_port")).toBe("ports");
    expect(containerFieldOf("restart_policy")).toBe("restartPolicy");
    expect(containerFieldOf("mounts.0")).toBeUndefined();
  });
});
