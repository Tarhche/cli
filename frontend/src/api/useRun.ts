import {useCallback, useState} from "react";
import {type Failure, failureOf, isCancelled} from "./failure";
import {useSignedOut} from "./signedOut";

/** How doing something went: done, with what it answered, or not. */
export type Outcome<T> = {done: true; value: T} | {done: false};

/** Something to do, whether it is being done, and why it failed if it did. */
export type Run<A extends unknown[], T> = {
  run: (...args: A) => Promise<Outcome<T>>;
  pending: boolean;
  failure: Failure | null;
  reset: () => void;
};

/**
 * Does action when asked to, remembering why it failed: a session that is
 * gone is said so, and a call that was cancelled is no failure.
 */
export function useRun<A extends unknown[], T>(action: (...args: A) => Promise<T>): Run<A, T> {
  const signedOut = useSignedOut();
  const [pending, setPending] = useState(false);
  const [failure, setFailure] = useState<Failure | null>(null);

  const run = useCallback(
    async (...args: A): Promise<Outcome<T>> => {
      setPending(true);
      setFailure(null);

      try {
        return {done: true, value: await action(...args)};
      } catch (error) {
        if (!isCancelled(error)) {
          const failed = failureOf(error);
          if (failed.logged_out) {
            signedOut(failed);
          }

          setFailure(failed);
        }

        return {done: false};
      } finally {
        setPending(false);
      }
    },
    [action, signedOut],
  );

  const reset = useCallback(() => setFailure(null), []);

  return {run, pending, failure, reset};
}
