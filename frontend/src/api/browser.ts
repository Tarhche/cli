import {Browser} from "@wailsio/runtime";
import type {MouseEvent} from "react";

/** Opens a page in the system's browser, which the app leaves the window to. */
export function openURL(url: string) {
  void Browser.OpenURL(url);
}

/**
 * Opens what a link points at in the system's browser, rather than in the
 * app's window, which following it would take away from the app.
 */
export function openInBrowser(event: MouseEvent, url: string) {
  event.preventDefault();
  openURL(url);
}
