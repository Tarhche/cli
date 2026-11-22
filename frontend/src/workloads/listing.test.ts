import {describe, expect, it, vi} from "vitest";
import {allPages} from "./listing";

describe("allPages", () => {
  it("reads every page, one after another", async () => {
    const read = vi.fn((page: number) => Promise.resolve({items: [page * 10, page * 10 + 1], pagination: {total_pages: 3, current_page: page}}));

    expect(await allPages(read)).toEqual([10, 11, 20, 21, 30, 31]);
    expect(read).toHaveBeenCalledTimes(3);
  });

  it("stops at a page with nothing on it", async () => {
    const read = vi.fn(() => Promise.resolve({items: null, pagination: {total_pages: 9, current_page: 1}}));

    expect(await allPages(read)).toEqual([]);
    expect(read).toHaveBeenCalledOnce();
  });

  it("reads one page of a listing that says no pages", async () => {
    const read = vi.fn(() => Promise.resolve({items: [1]}));

    expect(await allPages(read)).toEqual([1]);
  });
});
