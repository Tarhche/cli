import {notifications} from "@mantine/notifications";
import {useCallback} from "react";
import {failureOf, isCancelled} from "./failure";
import {useSignedOut} from "./signedOut";

/** Does something, saying why in a notification titled title when it fails. */
export type Act = (title: string, action: () => Promise<unknown>) => Promise<boolean>;

/**
 * Something to do from a list or a menu, where there is no form to say a
 * failure on: it is said in a notification, and answers whether it was done.
 */
export function useAct(): Act {
  const signedOut = useSignedOut();

  return useCallback(
    async (title: string, action: () => Promise<unknown>) => {
      try {
        await action();

        return true;
      } catch (error) {
        if (isCancelled(error)) {
          return false;
        }

        const failure = failureOf(error);
        if (failure.logged_out) {
          signedOut();
        }

        notifications.show({color: "red", title, message: failure.message});

        return false;
      }
    },
    [signedOut],
  );
}
