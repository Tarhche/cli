import {ActionIcon, Button, EmptyState, Group, Loader, Menu, Pagination, SegmentedControl, Table, Text} from "@mantine/core";
import {notifications} from "@mantine/notifications";
import {IconBraces, IconChevronLeft, IconChevronRight, IconDots, IconFileText, IconPencil, IconPlayerPlay, IconPlus, IconTrash} from "@tabler/icons-react";
import {useState} from "react";
import {type ResourceAction, type ResourceDefinition, Resources, type Row} from "../api/services";
import {useAct} from "../api/useAct";
import {useQuery} from "../api/useQuery";
import {DeleteModal} from "../components/DeleteModal";
import {FailureAlert} from "../components/FailureAlert";
import {JsonDrawer} from "../components/JsonDrawer";
import {PageHeader} from "../components/PageHeader";
import {ResourceEditor} from "./ResourceEditor";
import {ResourceFilters} from "./ResourceFilters";
import {actionsFor, type Audience, AUDIENCE_LABELS, audienceShown, audiencesOf, filtersFor, nameOf, routesFor, said} from "./resource";

/** What is being edited: something new, or a row. */
type Editing = {row: Row | null};

/** What is shown as JSON: a row, or what an action answered. */
type Shown = {title: string; value: unknown};

/**
 * A resource the dashboard manages, as its definition describes it for the
 * session: its listing, laid out as its table, of one's own things or
 * everybody's, as far as the session may list them, and what it may ask of
 * each of its rows.
 */
export function ResourcePage({definition}: {definition: ResourceDefinition}) {
  const act = useAct();
  const audiences = audiencesOf(definition);
  const [chosen, setAudience] = useState<Audience>(audiences[0] ?? "everyone");
  const audience = audienceShown(audiences, chosen);
  const [page, setPage] = useState(1);
  const [cursors, setCursors] = useState<string[]>([""]);
  const [filters, setFilters] = useState<Record<string, string>>({});
  const [shown, setShown] = useState<Shown | null>(null);
  const [editing, setEditing] = useState<Editing | null>(null);
  const [deleting, setDeleting] = useState<Row | null>(null);

  const everyone = audience === "everyone";
  const routes = routesFor(definition, audience);
  const cursor = cursors[cursors.length - 1] ?? "";
  const listing = useQuery(`resource:${definition.name}:${audience}:${page}:${cursor}:${JSON.stringify(filters)}`, () =>
    Resources.List(definition.name, everyone, {page, cursor, filters: said(filters)}),
  );
  const rows = listing.data?.rows ?? [];
  const columns = listing.data?.columns ?? definition.columns ?? [];
  const keyedActions = actionsFor(definition, audience, true);
  const resourceActions = actionsFor(definition, audience, false);

  const startOver = () => {
    setPage(1);
    setCursors([""]);
  };

  const inspect = (row: Row) => {
    if (!routes.get || row.key.length === 0) {
      setShown({title: nameOf(row.key, row.cells), value: row.item});

      return;
    }

    void act(`Reading ${nameOf(row.key, row.cells)}`, async () => {
      setShown({title: nameOf(row.key, row.cells), value: await Resources.Get(definition.name, everyone, row.key)});
    });
  };

  const ask = (action: ResourceAction, row: Row | null) =>
    void act(action.description || action.name, async () => {
      const answer = await Resources.Action(definition.name, action.name, everyone, row?.key ?? []);
      if (answer !== null && answer !== undefined) {
        setShown({title: action.description || action.name, value: answer});
      } else {
        notifications.show({color: "teal", message: `${action.description || action.name}: done`});
      }

      listing.reload();
    });

  return (
    <>
      <PageHeader
        title={definition.title}
        description={definition.description}
        actions={
          <>
            {resourceActions.map((action) => (
              <Button key={action.name} variant="default" leftSection={<IconPlayerPlay size={16} />} onClick={() => ask(action, null)}>
                {action.description || action.name}
              </Button>
            ))}
            {routes.create && (
              <Button leftSection={<IconPlus size={16} />} onClick={() => setEditing({row: null})}>
                Create
              </Button>
            )}
          </>
        }
      />
      {audiences.length > 1 && (
        <SegmentedControl
          mb="md"
          value={audience}
          onChange={(next) => {
            setAudience(next as Audience);
            startOver();
          }}
          data={audiences.map((a) => ({value: a, label: AUDIENCE_LABELS[a]}))}
        />
      )}
      <ResourceFilters
        key={audience}
        filters={filtersFor(definition, audience)}
        value={filters}
        onChange={(next) => {
          setFilters(next);
          startOver();
        }}
      />
      <FailureAlert failure={listing.failure} title={`The ${definition.title.toLowerCase()} could not be read`} />
      {listing.loading && <Loader />}
      {!listing.loading && rows.length === 0 && !listing.failure && <EmptyState icon={<IconFileText />} title={`No ${definition.title.toLowerCase()}`} />}
      {rows.length > 0 && (
        <Table.ScrollContainer minWidth={Math.max(480, columns.length * 140)}>
          <Table verticalSpacing="sm" highlightOnHover>
            <Table.Thead>
              <Table.Tr>
                {columns.map((column, i) => (
                  <Table.Th key={`${column}-${i}`}>{column}</Table.Th>
                ))}
                <Table.Th />
              </Table.Tr>
            </Table.Thead>
            <Table.Tbody>
              {rows.map((row, index) => (
                <Table.Tr key={row.key.length > 0 ? row.key.join("/") : index}>
                  {row.cells.map((cell, i) => (
                    <Table.Td key={i}>
                      <Text size="sm" style={{overflowWrap: "anywhere"}}>
                        {cell}
                      </Text>
                    </Table.Td>
                  ))}
                  <Table.Td>
                    <Group justify="flex-end">
                      <Menu position="bottom-end" withinPortal>
                        <Menu.Target>
                          <ActionIcon variant="subtle" color="gray" aria-label={`What to do with ${nameOf(row.key, row.cells)}`}>
                            <IconDots size={16} />
                          </ActionIcon>
                        </Menu.Target>
                        <Menu.Dropdown>
                          <Menu.Item leftSection={<IconBraces size={14} />} onClick={() => inspect(row)}>
                            Inspect
                          </Menu.Item>
                          {routes.update && row.key.length > 0 && (
                            <Menu.Item leftSection={<IconPencil size={14} />} onClick={() => setEditing({row})}>
                              Edit
                            </Menu.Item>
                          )}
                          {row.key.length > 0 &&
                            keyedActions.map((action) => (
                              <Menu.Item key={action.name} leftSection={<IconPlayerPlay size={14} />} onClick={() => ask(action, row)}>
                                {action.description || action.name}
                              </Menu.Item>
                            ))}
                          {routes.delete && row.key.length > 0 && (
                            <>
                              <Menu.Divider />
                              <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={() => setDeleting(row)}>
                                Delete
                              </Menu.Item>
                            </>
                          )}
                        </Menu.Dropdown>
                      </Menu>
                    </Group>
                  </Table.Td>
                </Table.Tr>
              ))}
            </Table.Tbody>
          </Table>
        </Table.ScrollContainer>
      )}
      {definition.cursor ? (
        (cursors.length > 1 || listing.data?.next) && (
          <Group justify="center" mt="md">
            <Button variant="default" leftSection={<IconChevronLeft size={16} />} disabled={cursors.length <= 1} onClick={() => setCursors((c) => c.slice(0, -1))}>
              Newer
            </Button>
            <Button variant="default" rightSection={<IconChevronRight size={16} />} disabled={!listing.data?.next} onClick={() => setCursors((c) => [...c, listing.data?.next ?? ""])}>
              Older
            </Button>
          </Group>
        )
      ) : (
        (listing.data?.total_pages ?? 0) > 1 && (
          <Group justify="center" mt="md">
            <Pagination total={listing.data?.total_pages ?? 1} value={page} onChange={setPage} />
          </Group>
        )
      )}
      <JsonDrawer opened={shown !== null} onClose={() => setShown(null)} title={shown?.title ?? ""} value={shown?.value} />
      <ResourceEditor
        opened={editing !== null}
        onClose={() => setEditing(null)}
        title={editing?.row ? `Edit ${nameOf(editing.row.key, editing.row.cells)}` : `New ${definition.name.replace(/-/g, " ")}`}
        read={editing?.row ? readerOf(definition, everyone, editing.row, routes.get) : undefined}
        save={(json) => (editing?.row ? Resources.Update(definition.name, everyone, editing.row.key, json) : Resources.Create(definition.name, everyone, json))}
        onSaved={() => {
          notifications.show({color: "teal", message: "Saved"});
          listing.reload();
        }}
      />
      <DeleteModal
        opened={deleting !== null}
        onClose={() => setDeleting(null)}
        title={`Delete ${definition.name.replace(/-/g, " ")}`}
        name={deleting ? nameOf(deleting.key, deleting.cells) : ""}
        onDelete={() => Resources.Delete(definition.name, everyone, deleting?.key ?? [])}
        onDeleted={listing.reload}
      />
    </>
  );
}

/** Reads a row as it is now, or answers it as it was listed where it cannot be read. */
function readerOf(definition: ResourceDefinition, everyone: boolean, row: Row, readable: boolean): () => Promise<unknown> {
  if (!readable) {
    return () => Promise.resolve(row.item);
  }

  return () => Resources.Get(definition.name, everyone, row.key);
}
