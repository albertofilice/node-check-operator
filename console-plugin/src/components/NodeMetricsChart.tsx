import React, { useState, useEffect } from 'react';
import { Card, CardBody, Spinner } from '@patternfly/react-core';
import { ChartDonutUtilization } from '@patternfly/react-charts/victory';
import { apiGet } from '../utils/api';

interface NodeCheckDetail {
  name: string;
  nodeName: string;
  systemResults?: {
    resources?: {
      details?: Record<string, any>;
      status?: string;
    };
    memory?: {
      details?: Record<string, any>;
      status?: string;
    };
    hardware?: {
      temperature?: {
        details?: Record<string, any>;
      };
    };
  };
  kubernetesResults?: {
    nodeResources?: {
      details?: Record<string, any>;
      status?: string;
    };
    nodeResourceUsage?: {
      details?: Record<string, any>;
      status?: string;
    };
  };
}

interface NodeMetricsChartProps {
  nodeChecks: Array<{ name: string; namespace: string; nodeName: string }>;
}

export const NodeMetricsChart: React.FC<NodeMetricsChartProps> = ({ nodeChecks }) => {
  const [nodeDetails, setNodeDetails] = useState<Record<string, NodeCheckDetail>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const loadDetails = async () => {
      try {
        setLoading(true);
        const details: Record<string, NodeCheckDetail> = {};
        
        // Carica i dettagli per ogni NodeCheck
        for (const nodeCheck of nodeChecks) {
          if (nodeCheck.nodeName === '*' || !nodeCheck.nodeName) continue;
          
          try {
            const namespace = nodeCheck.namespace || 'node-check-operator-system';
            const detail = await apiGet<NodeCheckDetail>(`nodechecks/${nodeCheck.name}`, { namespace });
            details[nodeCheck.nodeName] = detail;
          } catch (err) {
            console.error(`Errore caricamento dettagli per ${nodeCheck.name}:`, err);
          }
        }
        
        setNodeDetails(details);
      } catch (err) {
        console.error('Errore caricamento dettagli:', err);
      } finally {
        setLoading(false);
      }
    };

    if (nodeChecks.length > 0) {
      loadDetails();
    } else {
      setLoading(false);
    }
  }, [nodeChecks]);

  if (loading) {
    return (
      <Card>
        <CardBody>
          <div style={{ textAlign: 'center', padding: '2rem' }}>
            <Spinner size="lg" />
            <p style={{ marginTop: '1rem' }}>Caricamento metriche...</p>
          </div>
        </CardBody>
      </Card>
    );
  }

  // Estrai dati temperatura e calcola media
  const temperatureValues: number[] = [];
  Object.entries(nodeDetails).forEach(([nodeName, detail]) => {
    const temps = detail.systemResults?.hardware?.temperature?.details?.temperatures;
    if (temps && typeof temps === 'object') {
      // Calcola la media delle temperature per ogni nodo
      const tempValues = Object.values(temps).filter((v): v is number => typeof v === 'number');
      if (tempValues.length > 0) {
        const avgTemp = tempValues.reduce((sum, val) => sum + val, 0) / tempValues.length;
        temperatureValues.push(avgTemp);
      }
    }
  });

  // Estrai dati CPU e RAM ALLOCATE (da Kubernetes NodeResources check)
  const cpuAllocationValues: number[] = [];
  const ramAllocationValues: number[] = [];
  
  // Estrai dati CPU e RAM CONSUMI REAL-TIME (da Kubernetes NodeResourceUsage check)
  const cpuUsageValues: number[] = [];
  const ramUsageValues: number[] = [];
  
  Object.entries(nodeDetails).forEach(([nodeName, detail]) => {
    // ALLOCAZIONI: da kubernetesResults.nodeResources (requests/limits)
    const nodeResources = detail.kubernetesResults?.nodeResources?.details;
    if (nodeResources) {
      const percentages = nodeResources.percentages as Record<string, number> | undefined;
      if (percentages) {
        // CPU allocation (requests percent)
        if (typeof percentages.cpu_request_percent === 'number') {
          cpuAllocationValues.push(percentages.cpu_request_percent);
        }
        // RAM allocation (requests percent)
        if (typeof percentages.memory_request_percent === 'number') {
          ramAllocationValues.push(percentages.memory_request_percent);
        }
      }
    }
    
    // CONSUMI REAL-TIME: da kubernetesResults.nodeResourceUsage (metrics-server)
    const nodeResourceUsage = detail.kubernetesResults?.nodeResourceUsage?.details;
    if (nodeResourceUsage) {
      const cpuUsage = nodeResourceUsage.cpu_usage as Record<string, any> | undefined;
      const memoryUsage = nodeResourceUsage.memory_usage as Record<string, any> | undefined;
      
      if (cpuUsage && typeof cpuUsage.percent === 'number') {
        cpuUsageValues.push(cpuUsage.percent);
      } else {
        console.debug(`CPU usage not found or invalid for node ${nodeName}:`, cpuUsage);
      }
      
      if (memoryUsage && typeof memoryUsage.percent === 'number') {
        ramUsageValues.push(memoryUsage.percent);
      } else {
        console.debug(`Memory usage not found or invalid for node ${nodeName}:`, memoryUsage);
      }
    } else {
      console.debug(`NodeResourceUsage details not found for node ${nodeName}`);
    }
  });

  // Calcola medie (media di tutti i nodi)
  const avgTemperature = temperatureValues.length > 0 
    ? temperatureValues.reduce((sum, val) => sum + val, 0) / temperatureValues.length 
    : null;
  const avgCPUAllocation = cpuAllocationValues.length > 0 
    ? cpuAllocationValues.reduce((sum, val) => sum + val, 0) / cpuAllocationValues.length 
    : null;
  const avgRAMAllocation = ramAllocationValues.length > 0 
    ? ramAllocationValues.reduce((sum, val) => sum + val, 0) / ramAllocationValues.length 
    : null;
  const avgCPUUsage = cpuUsageValues.length > 0 
    ? cpuUsageValues.reduce((sum, val) => sum + val, 0) / cpuUsageValues.length 
    : null;
  const avgRAMUsage = ramUsageValues.length > 0 
    ? ramUsageValues.reduce((sum, val) => sum + val, 0) / ramUsageValues.length 
    : null;

  // Debug log per verificare i dati
  console.debug('NodeMetricsChart - Data summary:', {
    nodeCount: Object.keys(nodeDetails).length,
    temperatureValues: temperatureValues.length,
    cpuAllocationValues: cpuAllocationValues.length,
    ramAllocationValues: ramAllocationValues.length,
    cpuUsageValues: cpuUsageValues.length,
    ramUsageValues: ramUsageValues.length,
    avgCPUUsage,
    avgRAMUsage,
  });

  return (
    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: '1.5rem', marginTop: '1.5rem' }}>
      {/* Gauge Temperatura */}
      {avgTemperature !== null && (
        <Card>
          <CardBody>
            <div style={{ textAlign: 'center' }}>
              <h3 style={{ fontSize: '1.125rem', fontWeight: '600', marginBottom: '1rem' }}>Average Temperature</h3>
              <div style={{ height: '230px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                <ChartDonutUtilization
                  ariaDesc="Average node temperature"
                  ariaTitle="Average Temperature"
                  constrainToVisibleArea
                  data={{ x: 'Temperature', y: avgTemperature }}
                  labels={({ datum }) => datum.x ? `${datum.y.toFixed(1)}°C` : null}
                  subTitle="All Node Average"
                  title={`${avgTemperature.toFixed(1)}°C`}
                  thresholds={[
                    { value: 60, color: '#06c' },
                    { value: 70, color: '#ffc107' },
                    { value: 80, color: '#f0ab00' },
                    { value: 100, color: '#c9190b' },
                  ]}
                  height={230}
                  width={230}
                />
              </div>
            </div>
          </CardBody>
        </Card>
      )}

      {/* Gauge CPU Allocation */}
      {avgCPUAllocation !== null && (
        <Card>
          <CardBody>
            <div style={{ textAlign: 'center' }}>
              <h3 style={{ fontSize: '1.125rem', fontWeight: '600', marginBottom: '1rem' }}>CPU Allocation</h3>
              <div style={{ height: '230px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                <ChartDonutUtilization
                  ariaDesc="Average node CPU allocation based on pod requests"
                  ariaTitle="CPU Allocation"
                  constrainToVisibleArea
                  data={{ x: 'CPU', y: avgCPUAllocation }}
                  labels={({ datum }) => datum.x ? `${datum.y.toFixed(1)}%` : null}
                  subTitle="Allocated (Requests)"
                  title={`${avgCPUAllocation.toFixed(1)}%`}
                  thresholds={[
                    { value: 50, color: '#3e8635' },
                    { value: 75, color: '#ffc107' },
                    { value: 90, color: '#f0ab00' },
                    { value: 100, color: '#c9190b' },
                  ]}
                  height={230}
                  width={230}
                />
              </div>
            </div>
          </CardBody>
        </Card>
      )}

      {/* Gauge RAM Allocation */}
      {avgRAMAllocation !== null && (
        <Card>
          <CardBody>
            <div style={{ textAlign: 'center' }}>
              <h3 style={{ fontSize: '1.125rem', fontWeight: '600', marginBottom: '1rem' }}>RAM Allocation</h3>
              <div style={{ height: '230px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                <ChartDonutUtilization
                  ariaDesc="Average node RAM allocation based on pod requests"
                  ariaTitle="RAM Allocation"
                  constrainToVisibleArea
                  data={{ x: 'RAM', y: avgRAMAllocation }}
                  labels={({ datum }) => datum.x ? `${datum.y.toFixed(1)}%` : null}
                  subTitle="Allocated (Requests)"
                  title={`${avgRAMAllocation.toFixed(1)}%`}
                  thresholds={[
                    { value: 50, color: '#3e8635' },
                    { value: 75, color: '#ffc107' },
                    { value: 90, color: '#f0ab00' },
                    { value: 100, color: '#c9190b' },
                  ]}
                  height={230}
                  width={230}
                />
              </div>
            </div>
          </CardBody>
        </Card>
      )}

      {/* Gauge CPU Usage (Real-time) */}
      {avgCPUUsage !== null && (
        <Card>
          <CardBody>
            <div style={{ textAlign: 'center' }}>
              <h3 style={{ fontSize: '1.125rem', fontWeight: '600', marginBottom: '1rem' }}>CPU Usage (Real-time)</h3>
              <div style={{ height: '230px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                <ChartDonutUtilization
                  ariaDesc="Average node CPU real-time consumption"
                  ariaTitle="CPU Usage"
                  constrainToVisibleArea
                  data={{ x: 'CPU', y: avgCPUUsage }}
                  labels={({ datum }) => datum.x ? `${datum.y.toFixed(1)}%` : null}
                  subTitle="Actual Consumption"
                  title={`${avgCPUUsage.toFixed(1)}%`}
                  thresholds={[
                    { value: 50, color: '#3e8635' },
                    { value: 75, color: '#ffc107' },
                    { value: 90, color: '#f0ab00' },
                    { value: 100, color: '#c9190b' },
                  ]}
                  height={230}
                  width={230}
                />
              </div>
            </div>
          </CardBody>
        </Card>
      )}

      {/* Gauge RAM Usage (Real-time) */}
      {avgRAMUsage !== null && (
        <Card>
          <CardBody>
            <div style={{ textAlign: 'center' }}>
              <h3 style={{ fontSize: '1.125rem', fontWeight: '600', marginBottom: '1rem' }}>RAM Usage (Real-time)</h3>
              <div style={{ height: '230px', display: 'flex', justifyContent: 'center', alignItems: 'center' }}>
                <ChartDonutUtilization
                  ariaDesc="Average node RAM real-time consumption"
                  ariaTitle="RAM Usage"
                  constrainToVisibleArea
                  data={{ x: 'RAM', y: avgRAMUsage }}
                  labels={({ datum }) => datum.x ? `${datum.y.toFixed(1)}%` : null}
                  subTitle="Actual Consumption"
                  title={`${avgRAMUsage.toFixed(1)}%`}
                  thresholds={[
                    { value: 50, color: '#3e8635' },
                    { value: 75, color: '#ffc107' },
                    { value: 90, color: '#f0ab00' },
                    { value: 100, color: '#c9190b' },
                  ]}
                  height={230}
                  width={230}
                />
              </div>
            </div>
          </CardBody>
        </Card>
      )}

      {avgTemperature === null && avgCPUAllocation === null && avgRAMAllocation === null && avgCPUUsage === null && avgRAMUsage === null && (
        <Card>
          <CardBody>
            <p style={{ color: '#666', fontStyle: 'italic', textAlign: 'center', padding: '2rem' }}>
              No data available for charts. Ensure that NodeResources, NodeResourceUsage, and Temperature checks are enabled.
            </p>
          </CardBody>
        </Card>
      )}
    </div>
  );
};

