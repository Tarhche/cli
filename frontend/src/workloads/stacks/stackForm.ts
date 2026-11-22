import type {StackRequest} from "../../api/services";
import {type VMChoice, vmChoiceOf} from "../containers/containerForm";

/** What a compose file starts as: one service, which is enough to say how. */
export const COMPOSE_EXAMPLE = `services:
  web:
    image: nginx:latest
    ports:
      - "8080:80"
`;

/** What the stack form holds. */
export type StackForm = {
  name: string;
  compose: string;
  where: VMChoice;
};

/** The fields of the form, which what the API refuses is said beside. */
export type StackField = "name" | "compose" | "vm";

/** The form a new stack starts as, in vm when one is given. */
export function defaultStackForm(vm: string | null = null): StackForm {
  return {name: "", compose: COMPOSE_EXAMPLE, where: {fresh: vm === null, vm, name: ""}};
}

/** The stack the form asks for, as the API takes it. */
export function stackRequestOf(form: StackForm): StackRequest {
  return {name: form.name.trim(), compose: form.compose, ...vmChoiceOf(form.where)};
}

/**
 * The field of the form what the API refused of its field is said beside:
 * what is wrong in the compose file is refused under its path in it.
 */
export function stackFieldOf(field: string): StackField | undefined {
  if (field === "name") {
    return "name";
  }

  if (field === "compose" || field.startsWith("compose.")) {
    return "compose";
  }

  if (field === "vm" || field === "vm_uuid" || field.startsWith("vm.")) {
    return "vm";
  }

  return undefined;
}
