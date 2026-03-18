import ReactApexChart from 'react-apexcharts';
import type { ApexOptions } from 'apexcharts';
import { useMemo } from 'react';
import { useSymphonyStore } from '../../store/symphonyStore';
import type { TokenSample } from '../../store/symphonyStore';

function computeRates(samples: TokenSample[]): { x: number; y: number }[] {
  const rates: { x: number; y: number }[] = [];
  for (let i = 1; i < samples.length; i++) {
    const prev = samples[i - 1];
    const curr = samples[i];
    const deltaSec = Math.max((curr.ts - prev.ts) / 1000, 0.1);
    const deltaTokens = Math.max(curr.totalTokens - prev.totalTokens, 0);
    rates.push({ x: curr.ts, y: Math.round(deltaTokens / deltaSec) });
  }
  return rates;
}

interface Props {
  height?: number;
}

export default function TokenSparkline({ height = 60 }: Props) {
  const samples = useSymphonyStore((s) => s.tokenSamples);
  const rates = useMemo(() => computeRates(samples), [samples]);

  const options: ApexOptions = {
    chart: {
      type: 'area',
      sparkline: { enabled: true },
      animations: { enabled: false },
    },
    stroke: { curve: 'smooth', width: 2 },
    fill: {
      type: 'gradient',
      gradient: {
        shadeIntensity: 1,
        opacityFrom: 0.4,
        opacityTo: 0.05,
      },
    },
    colors: ['#3b82f6'],
    tooltip: {
      fixed: { enabled: false },
      x: { show: false },
      y: {
        formatter: (v: number) => `${String(v)} tok/s`,
      },
    },
    xaxis: { type: 'datetime' },
    yaxis: { min: 0 },
  };

  const series = [{ name: 'tok/s', data: rates }];

  if (rates.length < 2) {
    return (
      <div style={{ height }} className="flex items-end justify-center pb-1">
        <span className="text-xs text-gray-400 dark:text-gray-500">Collecting…</span>
      </div>
    );
  }

  return <ReactApexChart type="area" options={options} series={series} height={height} />;
}
