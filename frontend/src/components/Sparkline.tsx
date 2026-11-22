import {useMantineTheme} from "@mantine/core";

type Props = {
  /** The values, oldest first. */
  values: number[];

  /** The most a value may be, which the top of the line is; the largest value when none. */
  max?: number;

  color?: string;
  height?: number;
  label: string;
};

/** How values went over time, as a line. */
export function Sparkline({values, max, color = "blue", height = 48, label}: Props) {
  const theme = useMantineTheme();
  const width = 240;
  const top = Math.max(max ?? 0, ...values, 1);

  const points = values
    .map((value, i) => {
      const x = values.length === 1 ? width : (i / (values.length - 1)) * width;
      const y = height - (value / top) * (height - 2) - 1;

      return `${x.toFixed(1)},${y.toFixed(1)}`;
    })
    .join(" ");

  return (
    <svg role="img" aria-label={label} viewBox={`0 0 ${width} ${height}`} width="100%" height={height} preserveAspectRatio="none">
      <polyline points={points} fill="none" stroke={theme.colors[color]?.[6] ?? color} strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
    </svg>
  );
}
