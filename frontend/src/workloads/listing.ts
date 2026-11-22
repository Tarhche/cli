import {Workloads} from "../api/services";
import type {Page, VM} from "./types";

/** How often what a page shows is read again, while it is shown. */
export const POLL = 5000;

/** Every page of a listing, read one after another. */
export async function allPages<T>(read: (page: number) => Promise<Page<T>>): Promise<T[]> {
  const items: T[] = [];

  for (let page = 1; page <= 100; page++) {
    const answered = await read(page);
    items.push(...(answered.items ?? []));

    if ((answered.items ?? []).length === 0 || page >= (answered.pagination?.total_pages ?? 0)) {
      break;
    }
  }

  return items;
}

/** Every one of one's VMs. */
export function allVMs(): Promise<VM[]> {
  return allPages<VM>((page) => Workloads.VMs(page) as Promise<Page<VM>>);
}

/** Every one of one's Docker VMs, which containers and stacks run in. */
export async function dockerVMs(): Promise<VM[]> {
  return (await allVMs()).filter((vm) => vm.docker);
}
