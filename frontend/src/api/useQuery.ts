import {useCallback, useEffect, useRef, useState} from "react";
import {type Failure, failureOf, isCancelled} from "./failure";
import {useSignedOut} from "./signedOut";

/** What a query read, or why it could not. */
export type Query<T> = {
  data: T | undefined;
  failure: Failure | null;

  /** Whether it is reading what it has not read before. */
  loading: boolean;

  /** Reads it again, keeping what it read meanwhile. */
  reload: () => void;
};

export type QueryOptions = {
  /** Reads it again every so many milliseconds; none when zero. */
  interval?: number;

  /** Whether to read it at all. */
  enabled?: boolean;
};

type Read<T> = {
  key: string;
  data?: T;
  failure: Failure | null;
  loading: boolean;
};

/**
 * Reads what load answers, once for every key, and again whenever it is
 * reloaded or its interval passes. What an older read answers after a newer
 * one started is dropped, and a session that is gone is said so.
 */
export function useQuery<T>(key: string, load: () => Promise<T>, options: QueryOptions = {}): Query<T> {
  const {interval = 0, enabled = true} = options;
  const signedOut = useSignedOut();
  const latest = useRef(load);
  const [read, setRead] = useState<Read<T>>({key, failure: null, loading: true});
  const [round, setRound] = useState(0);

  useEffect(() => {
    latest.current = load;
  });

  useEffect(() => {
    if (!enabled) {
      return;
    }

    let current = true;

    latest.current().then(
      (data) => {
        if (current) {
          setRead({key, data, failure: null, loading: false});
        }
      },
      (error: unknown) => {
        if (!current || isCancelled(error)) {
          return;
        }

        const failure = failureOf(error);
        if (failure.logged_out) {
          signedOut(failure);
        }

        setRead((previous) => ({key, data: previous.key === key ? previous.data : undefined, failure, loading: false}));
      },
    );

    return () => {
      current = false;
    };
  }, [key, enabled, round, signedOut]);

  useEffect(() => {
    if (!enabled || interval <= 0) {
      return;
    }

    const timer = setInterval(() => setRound((r) => r + 1), interval);

    return () => clearInterval(timer);
  }, [enabled, interval]);

  const reload = useCallback(() => setRound((r) => r + 1), []);

  if (read.key !== key) {
    return {data: undefined, failure: null, loading: enabled, reload};
  }

  return {data: read.data, failure: read.failure, loading: enabled && read.loading, reload};
}
