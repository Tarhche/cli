import {ActionIcon, Button, EmptyState, Group, Loader, Menu, Pagination, SegmentedControl, Table, Text} from "@mantine/core";
import {modals} from "@mantine/modals";
import {notifications} from "@mantine/notifications";
import {
  IconChevronLeft,
  IconChevronRight,
  IconDots,
  IconExternalLink,
  IconEye,
  IconFileText,
  IconListCheck,
  IconPencil,
  IconPlayerPlay,
  IconPlus,
  type IconProps,
  IconTrash,
  IconWorld,
} from "@tabler/icons-react";
import {type ComponentType, useState} from "react";
import {openURL} from "../api/browser";
import {type ResourceAction, type ResourceDefinition, type ResourceLink, Resources, type Row} from "../api/services";
import {useAct} from "../api/useAct";
import {useQuery} from "../api/useQuery";
import {DeleteModal} from "../components/DeleteModal";
import {FailureAlert} from "../components/FailureAlert";
import {PageHeader} from "../components/PageHeader";
import {useNavigation} from "../shell/navigation";
import {ResourceFilters} from "./ResourceFilters";
import {actionsFor, type Audience, AUDIENCE_LABELS, audienceShown, audiencesOf, filtersFor, linksFor, nameOf, routesFor, rowActions, rowLinks, said} from "./resource";

/** The icon each kind of link is offered with. */
const LINK_ICONS: Record<string, ComponentType<IconProps>> = {
  view: IconEye,
  edit: IconPencil,
  list: IconListCheck,
  public: IconWorld,
};

type Props = {
  definition: ResourceDefinition;

  /** Whose things it opens on, while the session may list them. */
  audience?: Audience;

  /** What its listing opens narrowed by. */
  filters?: Record<string, string>;
};

/**
 * A resource the dashboard manages, as its definition describes it for the
 * session: its listing, laid out as its table, of one's own things or
 * everybody's, as far as the session may list them, and what the web
 * dashboard offers to do with each row. What needs a form of the
 * dashboard's own, making one, showing or changing it, opens that page of
 * the website in the browser.
 */
export function ResourcePage({definition, audience: openedOn, filters: narrowedBy = {}}: Props) {
  const act = useAct();
  const {navigate} = useNavigation();
  const audiences = audiencesOf(definition);
  const [chosen, setAudience] = useState<Audience>(openedOn ?? audiences[0] ?? "everyone");
  const audience = audienceShown(audiences, chosen);
  const [page, setPage] = useState(1);
  const [cursors, setCursors] = useState<string[]>([""]);
  const [filters, setFilters] = useState<Record<string, string>>(narrowedBy);
  const [deleting, setDeleting] = useState<Row | null>(null);

  const everyone = audience === "everyone";
  const routes = routesFor(definition, audience);
  const cursor = cursors[cursors.length - 1] ?? "";
  const listing = useQuery(`resource:${definition.name}:${audience}:${page}:${cursor}:${JSON.stringify(filters)}`, () =>
    Resources.List(definition.name, everyone, {page, cursor, filters: said(filters)}),
  );
  const rows = listing.data?.rows ?? [];
  const columns = listing.data?.columns ?? definition.columns ?? [];
  const links = linksFor(definition, audience);
  const keyedActions = actionsFor(definition, audience, true);
  const resourceActions = actionsFor(definition, audience, false);

  const startOver = () => {
    setPage(1);
    setCursors([""]);
  };

  const ask = (action: ResourceAction, row: Row | null) => {
    const run = () =>
      void act(action.title, async () => {
        await Resources.Action(definition.name, action.name, everyone, row?.key ?? []);
        notifications.show({color: "teal", message: `${action.title}: done`});
        listing.reload();
      });

    if (!action.confirm) {
      run();

      return;
    }

    modals.openConfirmModal({
      title: action.title,
      children: <Text size="sm">{action.confirm}</Text>,
      labels: {confirm: action.title, cancel: "Cancel"},
      onConfirm: run,
    });
  };

  const follow = (link: ResourceLink, to: string) => {
    if (link.resource) {
      navigate({page: "resource", name: link.resource, audience: "everyone", filters: {[link.filter]: to}});
    } else {
      openURL(to);
    }
  };

  return (
    <>
      <PageHeader
        title={definition.title}
        description={definition.description}
        actions={
          <>
            {resourceActions.map((action) => (
              <Button key={action.name} variant="default" leftSection={<IconPlayerPlay size={16} />} onClick={() => ask(action, null)}>
                {action.title}
              </Button>
            ))}
            {definition.new?.url && (
              <Button leftSection={<IconPlus size={16} />} rightSection={<IconExternalLink size={14} />} onClick={() => openURL(definition.new.url)}>
                {definition.new.name}
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
              {rows.map((row, index) => {
                const name = nameOf(row.key, row.cells);
                const followed = rowLinks(links, row);
                const asked = row.key.length > 0 ? rowActions(keyedActions, row) : [];
                const deletes = routes.delete && row.key.length > 0 && !row.protected;

                return (
                  <Table.Tr key={row.key.length > 0 ? row.key.join("/") : index}>
                    {row.cells.map((cell, i) => (
                      <Table.Td key={i}>
                        <Text size="sm" style={{overflowWrap: "anywhere"}}>
                          {cell}
                        </Text>
                      </Table.Td>
                    ))}
                    <Table.Td>
                      {(followed.length > 0 || asked.length > 0 || deletes) && (
                        <Group justify="flex-end">
                          <Menu position="bottom-end" withinPortal>
                            <Menu.Target>
                              <ActionIcon variant="subtle" color="gray" aria-label={`What to do with ${name}`}>
                                <IconDots size={16} />
                              </ActionIcon>
                            </Menu.Target>
                            <Menu.Dropdown>
                              {followed.map(({link, to}) => {
                                const Icon = LINK_ICONS[link.kind] ?? IconEye;

                                return (
                                  <Menu.Item
                                    key={link.name}
                                    leftSection={<Icon size={14} />}
                                    rightSection={link.resource ? undefined : <IconExternalLink size={12} aria-label="Opens in the browser" />}
                                    onClick={() => follow(link, to)}
                                  >
                                    {link.name}
                                  </Menu.Item>
                                );
                              })}
                              {asked.map((action) => (
                                <Menu.Item key={action.name} leftSection={<IconPlayerPlay size={14} />} onClick={() => ask(action, row)}>
                                  {action.title}
                                </Menu.Item>
                              ))}
                              {deletes && (
                                <>
                                  {(followed.length > 0 || asked.length > 0) && <Menu.Divider />}
                                  <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={() => setDeleting(row)}>
                                    Delete
                                  </Menu.Item>
                                </>
                              )}
                            </Menu.Dropdown>
                          </Menu>
                        </Group>
                      )}
                    </Table.Td>
                  </Table.Tr>
                );
              })}
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
