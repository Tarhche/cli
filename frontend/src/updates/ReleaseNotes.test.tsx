import {fireEvent, screen} from "@testing-library/react";
import {Browser} from "@wailsio/runtime";
import {describe, expect, it, vi} from "vitest";
import {render} from "../test/render";
import {ReleaseNotes} from "./ReleaseNotes";

describe("ReleaseNotes", () => {
  it("shows what a release says of itself", () => {
    render(<ReleaseNotes notes={"## Changelog\n### Features\n* 1a2b3c4 Feat: **upgrades** itself with `tarhche upgrade`"} />);

    expect(screen.getByRole("heading", {name: "Changelog"})).toBeInTheDocument();
    expect(screen.getByRole("heading", {name: "Features"})).toBeInTheDocument();
    expect(screen.getByRole("listitem")).toHaveTextContent("1a2b3c4 Feat: upgrades itself with tarhche upgrade");
    expect(screen.getByText("upgrades")).toBeInTheDocument();
    expect(screen.getByText("tarhche upgrade").tagName).toBe("CODE");
  });

  it("opens a link in the system's browser, not in the app's window", () => {
    render(<ReleaseNotes notes="**Full Changelog**: https://github.com/Tarhche/cli/compare/v0.1.0...v0.2.0" />);

    const link = screen.getByRole("link", {name: "https://github.com/Tarhche/cli/compare/v0.1.0...v0.2.0"});
    const followed = fireEvent.click(link);

    expect(followed).toBe(false);
    expect(vi.mocked(Browser.OpenURL)).toHaveBeenCalledWith("https://github.com/Tarhche/cli/compare/v0.1.0...v0.2.0");
  });

  it("makes nothing it says HTML", () => {
    const {container} = render(<ReleaseNotes notes={'<img src=x onerror="alert(1)"> <b>bold</b> [click](javascript:alert(1))'} />);

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector("b")).toBeNull();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
    expect(screen.getByText(/<img src=x onerror="alert\(1\)"> <b>bold<\/b>/)).toBeInTheDocument();
  });

  it("says when the release says nothing", () => {
    render(<ReleaseNotes notes="" />);

    expect(screen.getByText("The release says nothing of itself.")).toBeInTheDocument();
  });
});
