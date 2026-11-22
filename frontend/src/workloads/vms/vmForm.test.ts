import {describe, expect, it} from "vitest";
import {defaultVMForm, isPort, KIND_DEFAULTS, switchKind, vmFieldOf, vmRequestOf} from "./vmForm";

describe("the VM form", () => {
  it("starts as the least a machine is given", () => {
    const form = defaultVMForm();

    expect(form).toMatchObject({kind: "machine", image: "ubuntu:24.04", cpus: 1, memoryMiB: 128, diskGiB: 1, ports: [], ingress: true, egress: true, lifetimeSeconds: "0", snapshot: null});
    expect(defaultVMForm("s-1").snapshot).toBe("s-1");
  });

  it("takes another kind's start for what was left as it started, and keeps what was changed", () => {
    const docker = switchKind({...defaultVMForm(), memoryMiB: 2048}, "docker");

    expect(docker).toMatchObject({kind: "docker", cpus: 1, memoryMiB: 2048, diskGiB: 4, ports: ["80", "443", "8080"]});
    expect(switchKind(docker, "docker")).toBe(docker);
    expect(switchKind(docker, "machine")).toMatchObject({kind: "machine", memoryMiB: 2048, diskGiB: 1, ports: []});
    expect(KIND_DEFAULTS.docker.ports).toEqual(["80", "443", "8080"]);
  });

  it("asks for a VM in bytes, as the API takes it", () => {
    const request = vmRequestOf({
      ...defaultVMForm(),
      name: " box ",
      image: " debian:12 ",
      memoryMiB: 256,
      diskGiB: 2,
      ports: ["8080", "x", "22"],
      egress: false,
      persistentDisk: false,
      lifetimeSeconds: "3600",
    });

    expect(request).toEqual({
      name: "box",
      kind: "machine",
      image: "debian:12",
      resources: {cpus: 1, memory: 256 * 1024 * 1024, disk: 2 * 1024 ** 3},
      ports: [8080, 22],
      network: {ingress: "allow", egress: "deny"},
      persistent_disk: false,
      lifetime_seconds: 3600,
      snapshot_uuid: "",
    });
  });

  it("asks a Docker VM for no image, and a VM from a snapshot for it", () => {
    const request = vmRequestOf({...switchKind(defaultVMForm(), "docker"), snapshot: "s-1"});

    expect(request.image).toBe("");
    expect(request.snapshot_uuid).toBe("s-1");
  });

  it("takes only ports for ports", () => {
    expect(isPort("8080")).toBe(true);
    expect(isPort(" 22 ")).toBe(true);
    expect(isPort("http")).toBe(false);
    expect(isPort("123456")).toBe(false);
  });

  it("says what the API refused beside the field it is of", () => {
    expect(vmFieldOf("name")).toBe("name");
    expect(vmFieldOf("resources.memory")).toBe("memory");
    expect(vmFieldOf("resources")).toBe("memory");
    expect(vmFieldOf("ports.2")).toBe("ports");
    expect(vmFieldOf("network.egress")).toBe("egress");
    expect(vmFieldOf("lifetime_seconds")).toBe("lifetime");
    expect(vmFieldOf("snapshot_uuid")).toBe("snapshot");
    expect(vmFieldOf("something")).toBeUndefined();
  });
});
