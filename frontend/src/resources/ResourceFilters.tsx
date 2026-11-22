import {Button, Group, Paper, Select, TextInput} from "@mantine/core";
import {IconFilter} from "@tabler/icons-react";
import {type FormEvent, useState} from "react";
import type {ResourceFilter} from "../api/services";

type Props = {
  filters: ResourceFilter[];
  value: Record<string, string>;
  onChange: (value: Record<string, string>) => void;
};

/** What a listing is narrowed by, applied once asked. */
export function ResourceFilters({filters, value, onChange}: Props) {
  const [draft, setDraft] = useState(value);

  if (filters.length === 0) {
    return null;
  }

  const apply = (event: FormEvent) => {
    event.preventDefault();
    onChange(draft);
  };

  const set = (name: string, said: string) => setDraft((current) => ({...current, [name]: said}));

  return (
    <Paper withBorder p="sm" mb="md" component="form" onSubmit={apply}>
      <Group align="flex-end">
        {filters.map((filter) =>
          (filter.values ?? []).length > 0 ? (
            <Select
              key={filter.name}
              label={filter.name}
              description={filter.description}
              placeholder="Any"
              data={filter.values}
              value={draft[filter.name] || null}
              onChange={(chosen) => set(filter.name, chosen ?? "")}
              clearable
              w={200}
            />
          ) : (
            <TextInput
              key={filter.name}
              label={filter.name}
              description={filter.description}
              placeholder={filter.moment ? "2026-11-22T10:00:00Z" : "Any"}
              value={draft[filter.name] ?? ""}
              onChange={(event) => set(filter.name, event.currentTarget.value)}
              w={220}
            />
          ),
        )}
        <Button type="submit" variant="light" leftSection={<IconFilter size={16} />}>
          Filter
        </Button>
      </Group>
    </Paper>
  );
}
