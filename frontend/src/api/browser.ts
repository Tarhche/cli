import {Browser} from "@wailsio/runtime";
import type {MouseEvent} from "react";

/**
 * Opens what a link points at in the system's browser, rather than in the
 * app's window, which following it would take away from the app.
 */
export function openInBrowser(event: MouseEvent, url: string) {
  event.preventDefault();
  void Browser.OpenURL(url);
}
