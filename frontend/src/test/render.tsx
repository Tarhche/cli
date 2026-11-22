import {MantineProvider} from "@mantine/core";
import {ModalsProvider} from "@mantine/modals";
import {Notifications} from "@mantine/notifications";
import {render as renderWithin, type RenderResult} from "@testing-library/react";
import type {ReactElement} from "react";
import {theme} from "../theme";

/** Renders ui as the app does: themed, with modals and notifications. */
export function render(ui: ReactElement): RenderResult {
  return renderWithin(ui, {
    wrapper: ({children}) => (
      <MantineProvider theme={theme} env="test">
        <ModalsProvider>
          <Notifications />
          {children}
        </ModalsProvider>
      </MantineProvider>
    ),
  });
}
