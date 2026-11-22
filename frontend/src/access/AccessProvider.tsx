import {Button, Center, Loader, Stack} from "@mantine/core";
import {type ReactNode, useCallback, useEffect, useState} from "react";
import {type Failure, failureOf, isCancelled} from "../api/failure";
import {Resources, Workloads} from "../api/services";
import {useSignedOut} from "../api/signedOut";
import {FailureAlert} from "../components/FailureAlert";
import {useSession} from "../session/session";
import {type Access, AccessContext, sameAccess} from "./access";

/**
 * Reads what the session may see and do, and reads it again whenever the
 * permissions its token says it has change, which is how a role granted or
 * withdrawn since, seen at the token's next renewal, reaches the navigation
 * and the pages. What it read stays while it reads again, and is let go of
 * only once it says otherwise.
 */
export function AccessProvider({children}: {children: ReactNode}) {
  const session = useSession();
  const signedOut = useSignedOut();
  const [access, setAccess] = useState<Access | undefined>(undefined);
  const [failure, setFailure] = useState<Failure | null>(null);
  const [round, setRound] = useState(0);
  const permissions = JSON.stringify(session.status?.permissions ?? []);

  useEffect(() => {
    let current = true;

    Promise.all([Resources.Definitions(), Workloads.Abilities()]).then(
      ([definitions, abilities]) => {
        if (current) {
          const read = {definitions: definitions ?? [], abilities};
          setAccess((was) => (sameAccess(was, read) ? was : read));
          setFailure(null);
        }
      },
      (error: unknown) => {
        if (!current || isCancelled(error)) {
          return;
        }

        const failed = failureOf(error);
        if (failed.logged_out) {
          signedOut(failed);
        }

        setFailure(failed);
      },
    );

    return () => {
      current = false;
    };
  }, [permissions, round, signedOut]);

  const retry = useCallback(() => setRound((r) => r + 1), []);

  if (access === undefined) {
    return (
      <Center mih="100vh" p="md">
        {failure ? (
          <Stack maw={440} w="100%">
            <FailureAlert failure={failure} title="What you may do could not be read" />
            <Button variant="light" onClick={retry}>
              Try again
            </Button>
          </Stack>
        ) : (
          <Loader />
        )}
      </Center>
    );
  }

  return <AccessContext.Provider value={access}>{children}</AccessContext.Provider>;
}
