import React from 'react';
import { Card, Badge } from '@patternfly/react-core';
import { ChartDonut } from '@patternfly/react-charts/victory';

interface CircularChartData {
  label: string;
  value: number;
  color: string;
  onClick?: () => void;
}

interface CircularChartProps {
  title: string;
  badgeValue?: number;
  data: CircularChartData[];
  total: number;
  centerLabel: string;
  centerValue: string | number;
  centerSubLabel?: string;
}

export const CircularChart: React.FC<CircularChartProps> = ({
  title,
  badgeValue,
  data,
  total,
  centerLabel,
  centerValue,
  centerSubLabel,
}) => {
  // Prepara i dati per ChartDonut
  const chartData = data.map((item) => ({
    x: item.label,
    y: item.value,
  }));

  // Prepara i colori
  const colorScale = data.map((item) => item.color);

  // Prepara la legenda
  const legendData = data.map((item) => ({
    name: `${item.value} ${item.label}`,
  }));

  return (
    <Card>
      <div className="pf-v5-c-card__title" style={{ padding: 'var(--pf-v5-global--spacer--md)', paddingBottom: 'var(--pf-v5-global--spacer--sm)', borderBottom: '1px solid var(--pf-v5-global--BorderColor--100)' }}>
        <div className="pf-v5-c-card__title-text" style={{ fontSize: '1.125rem', fontWeight: '600', color: 'var(--pf-v5-global--Color--100)', lineHeight: '1.5' }}>
          {title}{badgeValue !== undefined && <span style={{ marginLeft: '0.75rem' }}><Badge isRead>{badgeValue}</Badge></span>}
        </div>
      </div>
      <div style={{ padding: 'var(--pf-v5-global--spacer--md)' }}>
          <div className="pf-v5-c-chart" style={{ position: 'relative' }}>
            <div style={{ height: '200px', width: '376px' }}>
              <ChartDonut
                ariaDesc={`Overview of ${title.toLowerCase()}`}
                ariaTitle={title}
                constrainToVisibleArea
                data={chartData}
                colorScale={colorScale}
                height={200}
                width={376}
                innerRadius={71}
                radius={80}
                title={`${centerValue}`}
                subTitle={centerSubLabel || centerLabel}
                legendData={legendData}
                legendOrientation="vertical"
                legendPosition="right"
                padding={{
                  bottom: 20,
                  left: 20,
                  right: 150,
                  top: 20,
                }}
                labels={({ datum }) => `${datum.y} ${datum.x}`}
                name={`chart-${title}`}
              />
            </div>
          </div>
        </div>
    </Card>
  );
};
