import React from 'react';
import {
  Card,
  CardBody,
} from '@patternfly/react-core';
import { StatusBadge } from './StatusBadge';
import { CircularChart } from './CircularChart';
import { navigateTo } from '../utils/navigation';
import { apiGet } from '../utils/api';
import { NodeMetricsChart } from './NodeMetricsChart';

interface CheckSummary {
  name: string;
  category: 'system' | 'kubernetes';
  enabled: boolean;
  healthyCount: number;
  warningCount: number;
  criticalCount: number;
  unknownCount: number;
  overallStatus: 'Healthy' | 'Warning' | 'Critical' | 'Unknown';
}

interface NodeCheck {
  name: string;
  namespace: string;
  nodeName: string;
}

export const StatsOverview: React.FC<{ stats: any; nodeChecks?: NodeCheck[] }> = ({ stats, nodeChecks = [] }) => {
  // Se stats è null o undefined, renderizza un messaggio invece di null
  if (!stats || typeof stats !== 'object') {
  return (
    <Card>
      <CardBody>
          <div style={{ padding: '1rem', textAlign: 'center', color: '#6c757d' }}>
            No statistics available
          </div>
      </CardBody>
    </Card>
  );
  }

  // Ensure all values are numbers with defaults
  // Support both old field names (totalNodes) and new ones (totalNodeChecks)
  const healthyNodes = typeof stats.healthyNodes === 'number' ? stats.healthyNodes : 0;
  const warningNodes = typeof stats.warningNodes === 'number' ? stats.warningNodes : 0;
  const criticalNodes = typeof stats.criticalNodes === 'number' ? stats.criticalNodes : 0;
  const unknownNodes = typeof stats.unknownNodes === 'number' ? stats.unknownNodes : 0;
  const totalNodeChecks = typeof stats.totalNodeChecks === 'number' ? stats.totalNodeChecks : 
                          (typeof (stats as any).totalNodes === 'number' ? (stats as any).totalNodes : 0);
  const lastUpdate = stats.lastUpdate ? new Date(stats.lastUpdate).toLocaleString() : '-';
  
  // Calcola i totali per i checks
  const totalHealthyChecks = stats.checks?.reduce((sum: number, check: CheckSummary) => sum + check.healthyCount, 0) || 0;
  const totalWarningChecks = stats.checks?.reduce((sum: number, check: CheckSummary) => sum + check.warningCount, 0) || 0;
  const totalCriticalChecks = stats.checks?.reduce((sum: number, check: CheckSummary) => sum + check.criticalCount, 0) || 0;
  const totalChecks = totalHealthyChecks + totalWarningChecks + totalCriticalChecks;

  // Calcola il numero totale di violazioni per i nodi
  const totalNodeViolations = warningNodes + criticalNodes;
  
  // Calcola il numero totale di violazioni per i checks
  const totalCheckViolations = totalWarningChecks + totalCriticalChecks;

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(400px, 1fr))', gap: '1.5rem' }}>
      {/* Grafico Nodi */}
      <CircularChart
        title="Node Status"
        badgeValue={totalNodeViolations}
        data={[
          {
            label: 'Critical',
            value: criticalNodes,
            color: 'var(--pf-v5-global--palette--red-200)',
            onClick: () => navigateTo('/nodecheck'),
          },
          {
            label: 'Warning',
            value: warningNodes,
            color: 'var(--pf-v5-global--palette--orange-300)',
            onClick: () => navigateTo('/nodecheck'),
          },
          {
            label: 'Unknown',
            value: unknownNodes,
            color: 'var(--pf-v5-global--palette--black-500)',
          },
          {
            label: 'Healthy',
            value: healthyNodes,
            color: 'var(--pf-v5-global--palette--blue-300)',
            onClick: () => navigateTo('/nodecheck'),
          },
        ]}
        total={totalNodeChecks}
        centerLabel="Nodes"
        centerValue={totalNodeViolations}
        centerSubLabel="Violations"
      />

      {/* Grafico Checks */}
      {stats.checks && stats.checks.length > 0 && (
        <CircularChart
          title="Check Status"
          badgeValue={totalCheckViolations}
          data={[
            {
              label: 'Critical',
              value: totalCriticalChecks,
              color: 'var(--pf-v5-global--palette--red-200)',
              onClick: () => navigateTo('/nodecheck'),
            },
            {
              label: 'Warning',
              value: totalWarningChecks,
              color: 'var(--pf-v5-global--palette--orange-300)',
              onClick: () => navigateTo('/nodecheck'),
            },
            {
              label: 'Healthy',
              value: totalHealthyChecks,
              color: 'var(--pf-v5-global--palette--blue-300)',
              onClick: () => navigateTo('/nodecheck'),
            },
          ]}
          total={totalChecks}
          centerLabel="Checks"
          centerValue={totalCheckViolations}
          centerSubLabel="Violations"
        />
      )}
      
      {/* Grafici Temperatura e CPU & RAM */}
      {nodeChecks && nodeChecks.length > 0 && (
        <div style={{ gridColumn: '1 / -1' }}>
          <NodeMetricsChart nodeChecks={nodeChecks} />
        </div>
      )}
    </div>
  );
};
