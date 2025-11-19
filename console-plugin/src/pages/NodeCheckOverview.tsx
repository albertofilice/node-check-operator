import React, { useState, useEffect } from 'react';
import {
  Card,
  CardBody,
  Spinner,
  Alert,
  Button,
  EmptyState,
  EmptyStateBody,
  Bullseye,
  Tabs,
  Tab,
  Checkbox,
  Select,
  SelectOption,
  SelectList,
  SelectOptionProps,
  MenuToggle,
  MenuToggleElement,
  TextInputGroup,
  TextInputGroupMain,
  TextInputGroupUtilities,
} from '@patternfly/react-core';
import TimesIcon from '@patternfly/react-icons/dist/esm/icons/times-icon';
import { Chip } from '@patternfly/react-core/deprecated';
import { Table, Thead, Tbody, Tr, Th, Td } from '@patternfly/react-table';
import { StatusBadge } from '../components/StatusBadge';
import { StatsOverview } from '../components/StatsCard';
import { apiGet } from '../utils/api';
import { navigateTo } from '../utils/navigation';
import '../styles.css';

interface NodeCheck {
  name: string;
  namespace: string;
  nodeName: string;
  overallStatus: 'Healthy' | 'Warning' | 'Critical' | 'Unknown';
  lastCheck: string;
  message: string;
}

interface CheckResult {
  status?: 'Healthy' | 'Warning' | 'Critical' | 'Unknown';
  message?: string;
  timestamp?: string;
  details?: Record<string, any>;
}

interface NodeCheckDetail {
  name: string;
  namespace: string;
  nodeName: string;
  overallStatus: 'Healthy' | 'Warning' | 'Critical' | 'Unknown';
  lastCheck: string;
  message: string;
  systemResults?: {
    uptime?: CheckResult;
    processes?: CheckResult;
    resources?: CheckResult;
    memory?: CheckResult;
    uninterruptibleTasks?: CheckResult;
    services?: CheckResult;
    systemLogs?: CheckResult;
    fileDescriptors?: CheckResult;
    zombieProcesses?: CheckResult;
    ntpSync?: CheckResult;
    kernelPanics?: CheckResult;
    oomKiller?: CheckResult;
    cpuFrequency?: CheckResult;
    interruptsBalance?: CheckResult;
    cpuStealTime?: CheckResult;
    memoryFragmentation?: CheckResult;
    swapActivity?: CheckResult;
    contextSwitches?: CheckResult;
    selinuxStatus?: CheckResult;
    sshAccess?: CheckResult;
    kernelModules?: CheckResult;
    hardware?: {
      temperature?: CheckResult;
      ipmi?: CheckResult;
      bmc?: CheckResult;
      fanStatus?: CheckResult;
      powerSupply?: CheckResult;
      memoryErrors?: CheckResult;
      pcieErrors?: CheckResult;
      cpuMicrocode?: CheckResult;
    };
    disks?: {
      space?: CheckResult;
      smart?: CheckResult;
      performance?: CheckResult;
      raid?: CheckResult;
      pvs?: CheckResult;
      lvm?: CheckResult;
      ioWait?: CheckResult;
      queueDepth?: CheckResult;
      filesystemErrors?: CheckResult;
      inodeUsage?: CheckResult;
      mountPoints?: CheckResult;
    };
    network?: {
      interfaces?: CheckResult;
      routing?: CheckResult;
      connectivity?: CheckResult;
      statistics?: CheckResult;
      errors?: CheckResult;
      latency?: CheckResult;
      dnsResolution?: CheckResult;
      bondingStatus?: CheckResult;
      firewallRules?: CheckResult;
    };
  };
  kubernetesResults?: {
    nodeStatus?: CheckResult;
    pods?: CheckResult;
    clusterOperators?: CheckResult;
    nodeResources?: CheckResult;
    nodeResourceUsage?: CheckResult;
    containerRuntime?: CheckResult;
    kubeletHealth?: CheckResult;
    cniPlugin?: CheckResult;
    nodeConditions?: CheckResult;
  };
}

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

interface Stats {
  totalNodeChecks: number;
  healthyNodes: number;
  warningNodes: number;
  criticalNodes: number;
  unknownNodes: number;
  lastUpdate: string;
  checks?: CheckSummary[];
}

const NodeCheckOverview: React.FC = () => {
  const [stats, setStats] = useState<Stats | null>(null);
  const [nodeChecks, setNodeChecks] = useState<NodeCheck[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<string | number>(0);
  const [expandedNodes, setExpandedNodes] = useState<Set<string>>(new Set());
  const [nodeDetails, setNodeDetails] = useState<Record<string, NodeCheckDetail>>({});
  const [nodeErrors, setNodeErrors] = useState<Record<string, string>>({});
  const [nodeLoading, setNodeLoading] = useState<Record<string, boolean>>({});
  const [nodeFilters, setNodeFilters] = useState<Record<string, { check: Set<string>, status: Set<'Healthy' | 'Warning' | 'Critical'> }>>({});
  const [expandedChecks, setExpandedChecks] = useState<Record<string, Set<string>>>({});
  const [nodeFilterDropdowns, setNodeFilterDropdowns] = useState<Record<string, boolean>>({});
  const [nodeCheckSearch, setNodeCheckSearch] = useState<Record<string, string>>({});
  const [nodeCheckTypeaheadOpen, setNodeCheckTypeaheadOpen] = useState<Record<string, boolean>>({});
  const [nodeCheckTypeaheadFocused, setNodeCheckTypeaheadFocused] = useState<Record<string, number | null>>({});
  const [nodeCheckTabs, setNodeCheckTabs] = useState<Record<string, string | number>>({});

  useEffect(() => {
    const loadData = async () => {
      try {
        setLoading(true);
        setError(null);

        const [statsData, nodeChecksData] = await Promise.all([
          apiGet<Stats>('stats'),
          apiGet<NodeCheck[]>('nodechecks'),
        ]);

        setStats(statsData);
        // Filter out generic NodeChecks (nodeName === "*")
        const filteredNodeChecks = (nodeChecksData || []).filter(
          (nc) => nc.nodeName !== "*"
        );
        setNodeChecks(filteredNodeChecks);
      } catch (err) {
        console.error('Errore caricamento dati:', err);
        setError(err instanceof Error ? err.message : 'Errore sconosciuto');
      } finally {
        setLoading(false);
      }
    };

    loadData();
  }, []);

  if (loading) {
    return (
      <div className="pf-v6-c-drawer__content">
        <div className="pf-v6-c-page__main-container">
          <main className="pf-v6-c-page__main">
            <section className="pf-v6-c-page__main-section">
              <div className="pf-v6-c-page__main-body">
                <Bullseye>
                  <div style={{ textAlign: 'center' }}>
                    <Spinner size="xl" />
                    <p style={{ marginTop: '1rem' }}>Caricamento dati...</p>
                  </div>
                </Bullseye>
              </div>
            </section>
          </main>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="pf-v6-c-drawer__content">
        <div className="pf-v6-c-page__main-container">
          <main className="pf-v6-c-page__main">
            <section className="pf-v6-c-page__main-section">
              <div className="pf-v6-c-page__main-body">
                <Alert variant="danger" title="Errore nel caricamento dei dati">
                  {error}
                </Alert>
              </div>
            </section>
          </main>
        </div>
      </div>
    );
  }

  // Raggruppa i NodeChecks per nodo
  const nodeGroups = nodeChecks.reduce((acc, nodeCheck) => {
    const nodeName = nodeCheck.nodeName || 'Unknown';
    if (!acc[nodeName]) {
      acc[nodeName] = [];
    }
    acc[nodeName].push(nodeCheck);
    return acc;
  }, {} as Record<string, NodeCheck[]>);

  // Funzione per caricare i dettagli di un nodo
  const loadNodeDetails = async (nodeName: string) => {
    // Trova il primo NodeCheck per questo nodo
    const firstNodeCheck = nodeChecks.find(nc => nc.nodeName === nodeName);
    if (!firstNodeCheck) {
      return;
    }

    try {
      setNodeLoading(prev => ({ ...prev, [nodeName]: true }));
      setNodeErrors(prev => {
        const newErrors = { ...prev };
        delete newErrors[nodeName];
        return newErrors;
      });
      const namespace = firstNodeCheck.namespace || 'node-check-operator-system';
      const detail = await apiGet<NodeCheckDetail>(`nodechecks/${firstNodeCheck.name}`, { namespace });
      setNodeDetails(prev => ({ ...prev, [nodeName]: detail }));
      // Inizializza i filtri per questo nodo
      setNodeFilters(prev => ({
        ...prev,
        [nodeName]: { check: new Set(), status: new Set() }
      }));
      setExpandedChecks(prev => ({
        ...prev,
        [nodeName]: new Set()
      }));
      // Inizializza il tab attivo per questo nodo
      setNodeCheckTabs(prev => ({
        ...prev,
        [nodeName]: 0
      }));
    } catch (err) {
      console.error(`Errore caricamento dettagli per nodo ${nodeName}:`, err);
      const errorMessage = err instanceof Error ? err.message : 'Errore sconosciuto nel caricamento dei dettagli';
      setNodeErrors(prev => ({ ...prev, [nodeName]: errorMessage }));
    } finally {
      setNodeLoading(prev => ({ ...prev, [nodeName]: false }));
    }
  };

  const toggleNode = async (nodeName: string) => {
    const newExpanded = new Set(expandedNodes);
    if (newExpanded.has(nodeName)) {
      newExpanded.delete(nodeName);
    } else {
      newExpanded.add(nodeName);
      // Carica i dettagli del NodeCheck quando si espande un nodo
      if (!nodeDetails[nodeName] && !nodeErrors[nodeName]) {
        await loadNodeDetails(nodeName);
      }
    }
    setExpandedNodes(newExpanded);
  };

  // Funzione helper per ottenere il nome del check dal titolo
  const getCheckNameFromTitle = (title: string): string => {
    const titleToCheckName: Record<string, string> = {
      'Processes': 'Processes',
      'Memory': 'Memory',
      'Temperature': 'Temperature',
      'Pods': 'Pods',
      'Uptime': 'Uptime',
      'Disk Space': 'Disk Space',
      'SMART': 'Disk SMART',
      'Disk Performance': 'Disk Performance',
      'RAID': 'RAID',
      'Interfaces': 'Network Interfaces',
      'Routing': 'Network Routing',
      'Connectivity': 'Network Connectivity',
      'Statistics': 'Network Statistics',
      'Services': 'Services',
      'Resources': 'Resources',
      'LVM Physical Volumes': 'LVM PVs',
      'LVM Logical Volumes': 'LVM',
      'Node Status': 'Node Status',
      'Cluster Operators': 'Cluster Operators',
      'Node Resources': 'Node Resources',
      'Node Resource Usage': 'Node Resource Usage',
      'Uninterruptible Tasks': 'Uninterruptible Tasks',
      'System Logs': 'System Logs',
      'IPMI': 'IPMI',
      'BMC': 'BMC',
      'File Descriptors': 'File Descriptors',
      'Zombie Processes': 'Zombie Processes',
      'NTP Sync': 'NTP Sync',
      'Kernel Panics': 'Kernel Panics',
      'OOM Killer': 'OOM Killer',
      'CPU Frequency': 'CPU Frequency',
      'Interrupts Balance': 'Interrupts Balance',
      'CPU Steal Time': 'CPU Steal Time',
      'Memory Fragmentation': 'Memory Fragmentation',
      'Swap Activity': 'Swap Activity',
      'Context Switches': 'Context Switches',
      'SELinux Status': 'SELinux Status',
      'SSH Access': 'SSH Access',
      'Kernel Modules': 'Kernel Modules',
      'Fan Status': 'Fan Status',
      'Power Supply': 'Power Supply',
      'Memory Errors': 'Memory Errors',
      'PCIe Errors': 'PCIe Errors',
      'CPU Microcode': 'CPU Microcode',
      'I/O Wait': 'I/O Wait',
      'Queue Depth': 'Queue Depth',
      'Filesystem Errors': 'Filesystem Errors',
      'Inode Usage': 'Inode Usage',
      'Mount Points': 'Mount Points',
      'Errors': 'Network Errors',
      'Latency': 'Network Latency',
      'DNS Resolution': 'DNS Resolution',
      'Bonding Status': 'Bonding Status',
      'Firewall Rules': 'Firewall Rules',
      'Container Runtime': 'Container Runtime',
      'Kubelet Health': 'Kubelet Health',
      'CNI Plugin': 'CNI Plugin',
      'Node Conditions': 'Node Conditions',
    };
    return titleToCheckName[title] || title;
  };

  const toggleCheck = (nodeName: string, checkKey: string) => {
    setExpandedChecks(prev => {
      const nodeExpanded = prev[nodeName] || new Set();
      const newSet = new Set(nodeExpanded);
      if (newSet.has(checkKey)) {
        newSet.delete(checkKey);
      } else {
        newSet.add(checkKey);
      }
      return { ...prev, [nodeName]: newSet };
    });
  };

  const handleFilterToggle = (nodeName: string, type: 'check' | 'status', value: string) => {
    setNodeFilters(prev => {
      const nodeFilter = prev[nodeName] || { check: new Set(), status: new Set() };
      if (type === 'check') {
        const newSet = new Set(nodeFilter.check);
        if (newSet.has(value)) {
          newSet.delete(value);
        } else {
          newSet.add(value);
        }
        return { ...prev, [nodeName]: { ...nodeFilter, check: newSet } };
      } else {
        const newSet = new Set(nodeFilter.status);
        const statusValue = value as 'Healthy' | 'Warning' | 'Critical';
        if (newSet.has(statusValue)) {
          newSet.delete(statusValue);
        } else {
          newSet.add(statusValue);
        }
        return { ...prev, [nodeName]: { ...nodeFilter, status: newSet } };
      }
    });
  };

  const handleFilterClear = (nodeName: string) => {
    setNodeFilters(prev => ({
      ...prev,
      [nodeName]: { check: new Set(), status: new Set() }
    }));
  };

  const renderCheckResult = (
    nodeName: string,
    title: string,
    result: CheckResult | undefined,
    checkKey: string,
    forceRender: boolean = false
  ) => {
    if (!result && !forceRender) return null;

    const displayResult: CheckResult = result || {
      status: 'Unknown',
      message: 'Check result not available',
      details: {},
    };

    const nodeFilter = nodeFilters[nodeName] || { check: new Set(), status: new Set() };

    // Se c'è un filtro per check, mostra solo quelli selezionati
    if (nodeFilter.check.size > 0) {
      const checkNameFromTitle = getCheckNameFromTitle(title);
      if (!nodeFilter.check.has(checkNameFromTitle)) {
        return null;
      }
    }

    // Se c'è un filtro per status, mostra solo quelli selezionati
    if (nodeFilter.status.size > 0) {
      const status = displayResult.status || 'Unknown';
      if (status === 'Unknown' || !nodeFilter.status.has(status)) {
        return null;
      }
    }

    const nodeExpanded = expandedChecks[nodeName] || new Set();
    const isExpanded = nodeExpanded.has(checkKey);
    const status = displayResult.status || 'Unknown';
    const message = displayResult.message || 'No message';
    const timestamp = displayResult.timestamp ? new Date(displayResult.timestamp).toLocaleString() : '-';
    const details = displayResult.details || {};
    const detailKeys = Object.keys(details);

    return (
      <Card key={checkKey} style={{ marginBottom: '1rem' }}>
        <CardBody>
          <div
            style={{
              display: 'flex',
              justifyContent: 'space-between',
              alignItems: 'center',
              cursor: 'pointer',
            }}
            onClick={() => toggleCheck(nodeName, checkKey)}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
              <button
                type="button"
                style={{
                  background: 'none',
                  border: 'none',
                  fontSize: '1rem',
                  cursor: 'pointer',
                  padding: '0.25rem',
                }}
                onClick={(e) => {
                  e.stopPropagation();
                  toggleCheck(nodeName, checkKey);
                }}
              >
                {isExpanded ? '▼' : '▶'}
              </button>
              <h3 style={{ fontSize: '1.125rem', margin: 0 }}>{title}</h3>
              <StatusBadge status={status} />
            </div>
            <span style={{ fontSize: '0.875rem', color: '#666' }}>
              Last Check: {timestamp}
            </span>
          </div>
          {isExpanded && (
            <div style={{ marginTop: '1rem', paddingLeft: '2rem' }}>
              <p>
                <strong>Status:</strong> <StatusBadge status={status} />
              </p>
              <p>
                <strong>Message:</strong> {message}
              </p>
              {detailKeys.length > 0 && (
                <>
                  <h4>Details:</h4>
                  <table style={{ width: '100%', borderCollapse: 'collapse' }}>
                    <tbody>
                      {detailKeys.map((key) => (
                        <tr key={key} style={{ borderBottom: '1px solid #eee' }}>
                          <td style={{ padding: '0.5rem', fontWeight: 'bold', width: '20%' }}>
                            {key}
                          </td>
                          <td style={{ padding: '0.5rem' }}>
                            {typeof details[key] === 'object' && details[key] !== null
                              ? JSON.stringify(details[key], null, 2)
                              : String(details[key])}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </>
              )}
            </div>
          )}
        </CardBody>
      </Card>
    );
  };

  const columns = ['Name', 'Node', 'Status', 'Last Check', 'Message'];
  const rows = nodeChecks.map((nodeCheck) => {
    if (!nodeCheck || !nodeCheck.name) {
      return null;
    }
    return {
      cells: [
        {
          title: nodeCheck.name || '-',
        },
        {
          title: nodeCheck.nodeName && nodeCheck.nodeName !== '*' ? (
            <button
              type="button"
              style={{ 
                background: 'none', 
                border: 'none', 
                color: 'var(--pf-v6-global--link--Color)', 
                textDecoration: 'none', 
                cursor: 'pointer',
                padding: 0,
                font: 'inherit'
              }}
              onMouseEnter={(e) => e.currentTarget.style.textDecoration = 'underline'}
              onMouseLeave={(e) => e.currentTarget.style.textDecoration = 'none'}
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                // Naviga alla tab Node Details e espandi il nodo
                setActiveTab(2);
                // Scroll al nodo e espandilo
                setTimeout(() => {
                  toggleNode(nodeCheck.nodeName);
                }, 100);
              }}
            >
              {nodeCheck.nodeName}
            </button>
          ) : (
            nodeCheck.nodeName || '-'
          ),
        },
        {
          title: nodeCheck.overallStatus ? (
            <StatusBadge status={nodeCheck.overallStatus} />
          ) : (
            <StatusBadge status="Unknown" />
          ),
        },
        {
          title: nodeCheck.lastCheck
            ? new Date(nodeCheck.lastCheck).toLocaleString()
            : '-',
        },
        {
          title: nodeCheck.message || '-',
        },
      ],
    };
  }).filter(Boolean);

  return (
    <div className="pf-v6-c-drawer__content">
      <div className="pf-v6-c-page__main-container">
        <main className="pf-v6-c-page__main">
          <section className="pf-v6-c-page__main-section">
            <div className="pf-v6-c-page__main-body">
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 'var(--pf-v6-global--spacer--md)' }}>
                <h1 
                  data-ouia-component-type="PF6/Content" 
                  data-ouia-safe="true" 
                  data-ouia-component-id="PageHeader-title" 
                  data-pf-content="true" 
                  className="pf-v6-c-content--h1"
                >
                  Node Check
                </h1>
                <Button
                  variant="secondary"
                  onClick={() => navigateTo('/k8s/all-namespaces/nodecheck.openshift.io~v1alpha1~NodeCheck')}
                >
                  Vedi NodeChecks (Kubernetes)
                </Button>
              </div>
            </div>
          </section>

          <section className="pf-v6-c-page__main-section">
            <div className="pf-v6-c-page__main-body">
              <Tabs
                activeKey={activeTab}
                onSelect={(event, tabIndex) => setActiveTab(tabIndex)}
              >
                <Tab eventKey={0} title="Overview">
                  <div style={{ marginTop: '1rem' }}>
              {stats ? <StatsOverview stats={stats} nodeChecks={nodeChecks} /> : null}
            </div>
                </Tab>

                <Tab eventKey={1} title="Checks">
                  <div style={{ marginTop: '1rem' }}>
              <Card>
                <CardBody>
                        {!stats?.checks || stats.checks.length === 0 ? (
                    <EmptyState>
                            <h4 style={{ fontSize: '1.25rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>
                              Nessun check disponibile
                            </h4>
                      <EmptyStateBody>
                              Non ci sono check disponibili. Verifica che l'operator sia configurato correttamente.
                      </EmptyStateBody>
                    </EmptyState>
                  ) : (
                    <div style={{ overflowX: 'auto' }}>
                            <table style={{ width: '100%', minWidth: '100%', borderCollapse: 'collapse' }}>
                              <thead>
                                <tr style={{ borderBottom: '2px solid #ccc' }}>
                                  <th style={{ padding: '0.75rem', textAlign: 'left' }}>Check</th>
                                  <th style={{ padding: '0.75rem', textAlign: 'left' }}>Category</th>
                                  <th style={{ padding: '0.75rem', textAlign: 'center' }}>Status</th>
                                  <th style={{ padding: '0.75rem', textAlign: 'center' }}>Healthy</th>
                                  <th style={{ padding: '0.75rem', textAlign: 'center' }}>Warning</th>
                                  <th style={{ padding: '0.75rem', textAlign: 'center' }}>Critical</th>
                                </tr>
                              </thead>
                              <tbody>
                                {stats.checks.map((check: CheckSummary, index: number) => (
                                  <tr key={index} style={{ borderBottom: '1px solid #eee' }}>
                                    <td style={{ padding: '0.75rem' }}><strong>{check.name}</strong></td>
                                    <td style={{ padding: '0.75rem', textTransform: 'capitalize' }}>{check.category}</td>
                                    <td style={{ padding: '0.75rem', textAlign: 'center' }}>
                                      <StatusBadge status={check.overallStatus} />
                                    </td>
                                    <td style={{ padding: '0.75rem', textAlign: 'center', color: '#28a745' }}>
                                      {check.healthyCount}
                                    </td>
                                    <td style={{ padding: '0.75rem', textAlign: 'center', color: '#ffc107' }}>
                                      {check.warningCount}
                                    </td>
                                    <td style={{ padding: '0.75rem', textAlign: 'center', color: '#dc3545' }}>
                                      {check.criticalCount}
                                    </td>
                                  </tr>
                                ))}
                              </tbody>
                            </table>
                          </div>
                        )}
                      </CardBody>
                    </Card>
                  </div>
                </Tab>

                <Tab eventKey={2} title="Node Details">
                  <div style={{ marginTop: '1rem' }}>
                    {Object.keys(nodeGroups).length === 0 ? (
                      <Card>
                        <CardBody>
                          <EmptyState>
                            <h4 style={{ fontSize: '1.25rem', fontWeight: 'bold', marginBottom: '0.5rem' }}>
                              Nessun nodo trovato
                            </h4>
                            <EmptyStateBody>
                              Non ci sono nodi disponibili. Verifica che l'operator sia configurato correttamente.
                            </EmptyStateBody>
                          </EmptyState>
                        </CardBody>
                      </Card>
                    ) : (
                      Object.entries(nodeGroups).map(([nodeName, checks]) => {
                        const isExpanded = expandedNodes.has(nodeName);
                        // Determina lo status del nodo basandosi sugli status dei suoi NodeChecks
                        const hasCritical = checks.some(c => c.overallStatus === 'Critical');
                        const hasWarning = checks.some(c => c.overallStatus === 'Warning');
                        const nodeStatus = hasCritical ? 'Critical' : hasWarning ? 'Warning' : checks.length > 0 ? checks[0].overallStatus : 'Unknown';
                        const healthyCount = checks.filter(c => c.overallStatus === 'Healthy').length;
                        const warningCount = checks.filter(c => c.overallStatus === 'Warning').length;
                        const criticalCount = checks.filter(c => c.overallStatus === 'Critical').length;

                        return (
                          <Card key={nodeName} style={{ marginBottom: '1rem' }}>
                            <CardBody>
                              <div
                                style={{
                                  display: 'flex',
                                  justifyContent: 'space-between',
                                  alignItems: 'center',
                                  cursor: 'pointer',
                                }}
                                onClick={() => toggleNode(nodeName)}
                              >
                                <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', flex: 1 }}>
                                  <span style={{ fontSize: '1rem', userSelect: 'none' }}>
                                    {isExpanded ? '▼' : '▶'}
                                  </span>
                                  <span
                                    style={{
                                      color: 'var(--pf-v6-global--link--Color)',
                                      fontWeight: '500',
                                      fontSize: '1.125rem',
                                    }}
                                  >
                                    {nodeName}
                                  </span>
                                  <StatusBadge status={nodeStatus} />
                                  <span style={{ fontSize: '0.875rem', color: '#666' }}>
                                    ({checks.length} NodeCheck{checks.length !== 1 ? 's' : ''})
                                  </span>
                                </div>
                                <div style={{ display: 'flex', gap: '1rem', fontSize: '0.875rem' }}>
                                  <span style={{ color: '#28a745' }}>Healthy: {healthyCount}</span>
                                  <span style={{ color: '#ffc107' }}>Warning: {warningCount}</span>
                                  <span style={{ color: '#dc3545' }}>Critical: {criticalCount}</span>
                                </div>
                              </div>
                              {isExpanded && (() => {
                                const nodeDetail = nodeDetails[nodeName];
                                const nodeFilter = nodeFilters[nodeName] || { check: new Set(), status: new Set() };
                                const availableChecks = [
                                  'Processes',
                                  'Memory',
                                  'Temperature',
                                  'Pods',
                                  'Uptime',
                                  'Disk Space',
                                  'Disk SMART',
                                  'Disk Performance',
                                  'RAID',
                                  'Network Interfaces',
                                  'Network Routing',
                                  'Network Connectivity',
                                  'Network Statistics',
                                  'Services',
                                  'Resources',
                                  'LVM PVs',
                                  'LVM',
                                  'Node Status',
                                  'Cluster Operators',
                                  'Node Resources',
                                  'Uninterruptible Tasks',
                                  'System Logs',
                                  'IPMI',
                                  'BMC',
                                ];

                                const isLoading = nodeLoading[nodeName];
                                const nodeError = nodeErrors[nodeName];
                                
                                if (isLoading) {
                                  return (
                                    <div style={{ marginTop: '1rem', paddingLeft: '2rem' }}>
                                      <Spinner size="sm" />
                                      <span style={{ marginLeft: '0.5rem' }}>Caricamento dettagli...</span>
                                    </div>
                                  );
                                }
                                
                                if (nodeError) {
                                  return (
                                    <div style={{ marginTop: '1rem', paddingLeft: '2rem' }}>
                                      <Alert variant="danger" title="Errore nel caricamento dei dettagli">
                                        {nodeError}
                                        <div style={{ marginTop: '0.5rem' }}>
                                          <Button
                                            variant="link"
                                            onClick={() => {
                                              // Rimuovi l'errore e i dettagli esistenti, poi ricarica
                                              setNodeErrors(prev => {
                                                const newErrors = { ...prev };
                                                delete newErrors[nodeName];
                                                return newErrors;
                                              });
                                              setNodeDetails(prev => {
                                                const newDetails = { ...prev };
                                                delete newDetails[nodeName];
                                                return newDetails;
                                              });
                                              // Ricarica i dettagli
                                              loadNodeDetails(nodeName);
                                            }}
                                          >
                                            Riprova
                                          </Button>
                                        </div>
                                      </Alert>
                                    </div>
                                  );
                                }
                                
                                if (!nodeDetail) {
                                  return (
                                    <div style={{ marginTop: '1rem', paddingLeft: '2rem' }}>
                                      <Alert variant="warning" title="Dettagli non disponibili">
                                        I dettagli per questo nodo non sono ancora stati caricati.
                                      </Alert>
                                    </div>
                                  );
                                }

                                const systemResults = nodeDetail.systemResults;
                                const kubernetesResults = nodeDetail.kubernetesResults;
                                const hasSystemResults = systemResults && (
                                  systemResults.uptime || systemResults.processes || systemResults.resources ||
                                  systemResults.memory || systemResults.services || systemResults.systemLogs ||
                                  systemResults.uninterruptibleTasks || systemResults.fileDescriptors ||
                                  systemResults.zombieProcesses || systemResults.ntpSync || systemResults.kernelPanics ||
                                  systemResults.oomKiller || systemResults.cpuFrequency ||
                                  systemResults.interruptsBalance || systemResults.cpuStealTime || systemResults.memoryFragmentation ||
                                  systemResults.swapActivity || systemResults.contextSwitches || systemResults.selinuxStatus ||
                                  systemResults.sshAccess || systemResults.kernelModules ||
                                  (systemResults.hardware && (systemResults.hardware.temperature || systemResults.hardware.ipmi || systemResults.hardware.bmc ||
                                    systemResults.hardware.fanStatus || systemResults.hardware.powerSupply || systemResults.hardware.memoryErrors ||
                                    systemResults.hardware.pcieErrors || systemResults.hardware.cpuMicrocode)) ||
                                  (systemResults.disks && (systemResults.disks.space || systemResults.disks.smart || systemResults.disks.performance ||
                                    systemResults.disks.raid || systemResults.disks.pvs || systemResults.disks.lvm || systemResults.disks.ioWait ||
                                    systemResults.disks.queueDepth || systemResults.disks.filesystemErrors || systemResults.disks.inodeUsage ||
                                    systemResults.disks.mountPoints)) ||
                                  (systemResults.network && (systemResults.network.interfaces || systemResults.network.routing ||
                                    systemResults.network.connectivity || systemResults.network.statistics || systemResults.network.errors ||
                                    systemResults.network.latency || systemResults.network.dnsResolution || systemResults.network.bondingStatus ||
                                    systemResults.network.firewallRules))
                                );
                                const hasKubernetesResults = kubernetesResults && (
                                  kubernetesResults.nodeStatus || kubernetesResults.pods ||
                                  kubernetesResults.clusterOperators || kubernetesResults.nodeResources ||
                                  kubernetesResults.nodeResourceUsage || kubernetesResults.containerRuntime ||
                                  kubernetesResults.kubeletHealth || kubernetesResults.cniPlugin ||
                                  kubernetesResults.nodeConditions
                                );

                                const isFilterDropdownOpen = nodeFilterDropdowns[nodeName] || false;
                                const checkSearchValue = nodeCheckSearch[nodeName] || '';
                                const isCheckTypeaheadOpen = nodeCheckTypeaheadOpen[nodeName] || false;
                                const checkTypeaheadFocused = nodeCheckTypeaheadFocused[nodeName] || null;
                                
                                // Filtra i check disponibili in base alla ricerca
                                const filteredAvailableChecks = availableChecks
                                  .filter(check => !nodeFilter.check.has(check)) // Escludi quelli già selezionati
                                  .filter(check =>
                                    check.toLowerCase().includes(checkSearchValue.toLowerCase())
                                  );
                                
                                const checkTypeaheadOptions: SelectOptionProps[] = filteredAvailableChecks.map(check => ({
                                  value: check,
                                  children: check
                                }));

                                return (
                                  <div style={{ marginTop: '1rem', paddingLeft: '2rem' }}>
                                    {/* Informazioni del nodo */}
                                    <Card style={{ marginBottom: '1rem' }}>
                                      <CardBody>
                                        <div style={{ overflowX: 'auto' }}>
                                          <table style={{ width: '100%', borderCollapse: 'collapse', marginBottom: '1rem' }}>
                                            <tbody>
                                              <tr style={{ borderBottom: '1px solid #eee' }}>
                                                <td style={{ padding: '0.75rem', fontWeight: 'bold', width: '20%' }}>Name</td>
                                                <td style={{ padding: '0.75rem' }}>{nodeDetail.name}</td>
                                              </tr>
                                              <tr style={{ borderBottom: '1px solid #eee' }}>
                                                <td style={{ padding: '0.75rem', fontWeight: 'bold' }}>Namespace</td>
                                                <td style={{ padding: '0.75rem' }}>{nodeDetail.namespace}</td>
                                              </tr>
                                              <tr style={{ borderBottom: '1px solid #eee' }}>
                                                <td style={{ padding: '0.75rem', fontWeight: 'bold' }}>Node</td>
                                                <td style={{ padding: '0.75rem' }}>{nodeDetail.nodeName || '-'}</td>
                                              </tr>
                                              <tr style={{ borderBottom: '1px solid #eee' }}>
                                                <td style={{ padding: '0.75rem', fontWeight: 'bold' }}>Status</td>
                                                <td style={{ padding: '0.75rem' }}>
                                                  <StatusBadge status={nodeDetail.overallStatus} />
                                                </td>
                                              </tr>
                                              <tr style={{ borderBottom: '1px solid #eee' }}>
                                                <td style={{ padding: '0.75rem', fontWeight: 'bold' }}>Last Check</td>
                                                <td style={{ padding: '0.75rem' }}>
                                                  {nodeDetail.lastCheck ? new Date(nodeDetail.lastCheck).toLocaleString() : '-'}
                                                </td>
                                              </tr>
                                              <tr style={{ borderBottom: '1px solid #eee' }}>
                                                <td style={{ padding: '0.75rem', fontWeight: 'bold' }}>Message</td>
                                                <td style={{ padding: '0.75rem' }}>{nodeDetail.message || '-'}</td>
                                              </tr>
                                            </tbody>
                                          </table>
                                        </div>
                                      </CardBody>
                                    </Card>

                                    {/* Barra filtri */}
                                    <div style={{ marginBottom: '1rem', display: 'flex', gap: '0.5rem', alignItems: 'center', flexWrap: 'wrap' }}>
                                      {/* Pulsante Filter con dropdown per Status */}
                                      <Select
                                        isOpen={isFilterDropdownOpen}
                                        onOpenChange={(isOpen) => setNodeFilterDropdowns(prev => ({ ...prev, [nodeName]: isOpen }))}
                                        onSelect={(event, value) => {
                                          if (value) {
                                            handleFilterToggle(nodeName, 'status', value as string);
                                          }
                                          setNodeFilterDropdowns(prev => ({ ...prev, [nodeName]: false }));
                                        }}
                                        selected=""
                                        toggle={(toggleRef: React.Ref<MenuToggleElement>) => (
                                          <MenuToggle
                                            ref={toggleRef}
                                            onClick={() => setNodeFilterDropdowns(prev => ({ ...prev, [nodeName]: !prev[nodeName] }))}
                                            isExpanded={isFilterDropdownOpen}
                                            variant="default"
                                            style={{ minWidth: '120px' }}
                                          >
                                            Filter
                                          </MenuToggle>
                                        )}
                                      >
                                        <SelectList>
                                          <SelectOption value="Healthy">Healthy</SelectOption>
                                          <SelectOption value="Warning">Warning</SelectOption>
                                          <SelectOption value="Critical">Critical</SelectOption>
                                        </SelectList>
                                      </Select>

                                      {/* Campo typeahead per cercare i check */}
                                      <Select
                                        id={`${nodeName}-check-typeahead`}
                                        isOpen={isCheckTypeaheadOpen}
                                        selected=""
                                        onSelect={(event, value) => {
                                          if (value && typeof value === 'string') {
                                            handleFilterToggle(nodeName, 'check', value);
                                            setNodeCheckSearch(prev => ({ ...prev, [nodeName]: '' }));
                                            setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: false }));
                                            setNodeCheckTypeaheadFocused(prev => ({ ...prev, [nodeName]: null }));
                                          }
                                        }}
                                        onOpenChange={(isOpen) => {
                                          setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: isOpen }));
                                          if (!isOpen) {
                                            setNodeCheckTypeaheadFocused(prev => ({ ...prev, [nodeName]: null }));
                                          }
                                        }}
                                        variant="typeahead"
                                        toggle={(toggleRef: React.Ref<MenuToggleElement>) => (
                                          <MenuToggle
                                            ref={toggleRef}
                                            variant="typeahead"
                                            onClick={() => {
                                              setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: !prev[nodeName] }));
                                            }}
                                            isExpanded={isCheckTypeaheadOpen}
                                            style={{ minWidth: '250px' }}
                                          >
                                            <TextInputGroup isPlain>
                                              <TextInputGroupMain
                                                value={checkSearchValue}
                                                onChange={(_, value) => {
                                                  setNodeCheckSearch(prev => ({ ...prev, [nodeName]: value }));
                                                  if (value && !isCheckTypeaheadOpen) {
                                                    setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: true }));
                                                  }
                                                  setNodeCheckTypeaheadFocused(prev => ({ ...prev, [nodeName]: null }));
                                                }}
                                                onKeyDown={(e) => {
                                                  if (e.key === 'Enter' && checkTypeaheadFocused !== null && checkTypeaheadOptions[checkTypeaheadFocused]) {
                                                    const selectedOption = checkTypeaheadOptions[checkTypeaheadFocused];
                                                    handleFilterToggle(nodeName, 'check', selectedOption.value as string);
                                                    setNodeCheckSearch(prev => ({ ...prev, [nodeName]: '' }));
                                                    setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: false }));
                                                    setNodeCheckTypeaheadFocused(prev => ({ ...prev, [nodeName]: null }));
                                                  } else if (e.key === 'ArrowDown') {
                                                    e.preventDefault();
                                                    const nextIndex = checkTypeaheadFocused === null ? 0 : Math.min(checkTypeaheadFocused + 1, checkTypeaheadOptions.length - 1);
                                                    setNodeCheckTypeaheadFocused(prev => ({ ...prev, [nodeName]: nextIndex }));
                                                    setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: true }));
                                                  } else if (e.key === 'ArrowUp') {
                                                    e.preventDefault();
                                                    const prevIndex = checkTypeaheadFocused === null ? checkTypeaheadOptions.length - 1 : Math.max(checkTypeaheadFocused - 1, 0);
                                                    setNodeCheckTypeaheadFocused(prev => ({ ...prev, [nodeName]: prevIndex }));
                                                    setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: true }));
                                                  }
                                                }}
                                                placeholder="Search by check name..."
                                                autoComplete="off"
                                                role="combobox"
                                                isExpanded={isCheckTypeaheadOpen}
                                                aria-controls={`${nodeName}-check-typeahead-listbox`}
                                              />
                                              {checkSearchValue && (
                                                <TextInputGroupUtilities>
                                                  <Button
                                                    variant="plain"
                                                    onClick={() => {
                                                      setNodeCheckSearch(prev => ({ ...prev, [nodeName]: '' }));
                                                      setNodeCheckTypeaheadOpen(prev => ({ ...prev, [nodeName]: false }));
                                                    }}
                                                    aria-label="Clear input value"
                                                    icon={<TimesIcon />}
                                                  />
                                                </TextInputGroupUtilities>
                                              )}
                                            </TextInputGroup>
                                          </MenuToggle>
                                        )}
                                      >
                                        <SelectList id={`${nodeName}-check-typeahead-listbox`}>
                                          {checkTypeaheadOptions.length > 0 ? (
                                            checkTypeaheadOptions.map((option, index) => (
                                              <SelectOption
                                                key={option.value as string}
                                                value={option.value}
                                                isFocused={checkTypeaheadFocused === index}
                                              >
                                                {option.children}
                                              </SelectOption>
                                            ))
                                          ) : (
                                            <SelectOption isDisabled value="no-results">
                                              {checkSearchValue ? `No results found for "${checkSearchValue}"` : 'No checks available'}
                                            </SelectOption>
                                          )}
                                        </SelectList>
                                      </Select>

                                      {/* Link Clear all filters */}
                                      {(nodeFilter.check.size > 0 || nodeFilter.status.size > 0) && (
                                        <Button
                                          variant="link"
                                          onClick={() => {
                                            handleFilterClear(nodeName);
                                            setNodeCheckSearch(prev => ({ ...prev, [nodeName]: '' }));
                                          }}
                                          style={{ marginLeft: 'auto' }}
                                        >
                                          Clear all filters
                                        </Button>
                                      )}
                                    </div>

                                    {/* Tag dei filtri selezionati */}
                                    {(nodeFilter.check.size > 0 || nodeFilter.status.size > 0) && (
                                      <div style={{ marginBottom: '1rem', display: 'flex', gap: '0.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
                                        {Array.from(nodeFilter.status).map((status) => (
                                          <div key={`${nodeName}-status-${status}`} style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                                            <span style={{ fontSize: '0.875rem', color: '#666' }}>Alert State:</span>
                                            <Chip
                                              onClick={() => handleFilterToggle(nodeName, 'status', status)}
                                              style={{ cursor: 'pointer' }}
                                            >
                                              {status}
                                            </Chip>
                                          </div>
                                        ))}
                                        {Array.from(nodeFilter.check).map((check) => (
                                          <div key={`${nodeName}-check-${check}`} style={{ display: 'flex', alignItems: 'center', gap: '0.25rem' }}>
                                            <span style={{ fontSize: '0.875rem', color: '#666' }}>Check:</span>
                                            <Chip
                                              onClick={() => handleFilterToggle(nodeName, 'check', check)}
                                              style={{ cursor: 'pointer' }}
                                            >
                                              {check}
                                            </Chip>
                                          </div>
                                        ))}
                    </div>
                  )}

                                    {/* Check dettagliati */}
                                    <div>
                                      <Tabs 
                                        activeKey={nodeCheckTabs[nodeName] ?? 0} 
                                        onSelect={(event, tabIndex) => {
                                          setNodeCheckTabs(prev => ({
                                            ...prev,
                                            [nodeName]: tabIndex
                                          }));
                                        }}
                                      >
                                          <Tab eventKey={0} title="System Checks">
                                            <div style={{ marginTop: '1rem' }}>
                                              {systemResults ? (
                                                <>
                                                  {renderCheckResult(nodeName, 'Uptime', systemResults.uptime, `${nodeName}-system-uptime`, true)}
                                                  {renderCheckResult(nodeName, 'Processes', systemResults.processes, `${nodeName}-system-processes`, true)}
                                                  {renderCheckResult(nodeName, 'Resources', systemResults.resources, `${nodeName}-system-resources`, true)}
                                                  {renderCheckResult(nodeName, 'Memory', systemResults.memory, `${nodeName}-system-memory`, true)}
                                                  {renderCheckResult(nodeName, 'Uninterruptible Tasks', systemResults.uninterruptibleTasks, `${nodeName}-system-uninterruptible-tasks`, true)}
                                                  {renderCheckResult(nodeName, 'Services', systemResults.services, `${nodeName}-system-services`, true)}
                                                  {renderCheckResult(nodeName, 'System Logs', systemResults.systemLogs, `${nodeName}-system-logs`, true)}
                                                  {renderCheckResult(nodeName, 'File Descriptors', systemResults.fileDescriptors, `${nodeName}-system-file-descriptors`, true)}
                                                  {renderCheckResult(nodeName, 'Zombie Processes', systemResults.zombieProcesses, `${nodeName}-system-zombie-processes`, true)}
                                                  {renderCheckResult(nodeName, 'NTP Sync', systemResults.ntpSync, `${nodeName}-system-ntp-sync`, true)}
                                                  {renderCheckResult(nodeName, 'Kernel Panics', systemResults.kernelPanics, `${nodeName}-system-kernel-panics`, true)}
                                                  {renderCheckResult(nodeName, 'OOM Killer', systemResults.oomKiller, `${nodeName}-system-oom-killer`, true)}
                                                  {renderCheckResult(nodeName, 'CPU Frequency', systemResults.cpuFrequency, `${nodeName}-system-cpu-frequency`, true)}
                                                  {renderCheckResult(nodeName, 'Interrupts Balance', systemResults.interruptsBalance, `${nodeName}-system-interrupts-balance`, true)}
                                                  {renderCheckResult(nodeName, 'CPU Steal Time', systemResults.cpuStealTime, `${nodeName}-system-cpu-steal-time`, true)}
                                                  {renderCheckResult(nodeName, 'Memory Fragmentation', systemResults.memoryFragmentation, `${nodeName}-system-memory-fragmentation`, true)}
                                                  {renderCheckResult(nodeName, 'Swap Activity', systemResults.swapActivity, `${nodeName}-system-swap-activity`, true)}
                                                  {renderCheckResult(nodeName, 'Context Switches', systemResults.contextSwitches, `${nodeName}-system-context-switches`, true)}
                                                  {renderCheckResult(nodeName, 'SELinux Status', systemResults.selinuxStatus, `${nodeName}-system-selinux-status`, true)}
                                                  {renderCheckResult(nodeName, 'SSH Access', systemResults.sshAccess, `${nodeName}-system-ssh-access`, true)}
                                                  {renderCheckResult(nodeName, 'Kernel Modules', systemResults.kernelModules, `${nodeName}-system-kernel-modules`, true)}
                                                  
                                                  <h3 style={{ fontSize: '1.2rem', marginTop: '1.5rem', marginBottom: '0.5rem' }}>Hardware</h3>
                                                  {renderCheckResult(nodeName, 'Temperature', systemResults.hardware?.temperature, `${nodeName}-hardware-temperature`, true)}
                                                  {renderCheckResult(nodeName, 'IPMI', systemResults.hardware?.ipmi, `${nodeName}-hardware-ipmi`, true)}
                                                  {renderCheckResult(nodeName, 'BMC', systemResults.hardware?.bmc, `${nodeName}-hardware-bmc`, true)}
                                                  {renderCheckResult(nodeName, 'Fan Status', systemResults.hardware?.fanStatus, `${nodeName}-hardware-fan-status`, true)}
                                                  {renderCheckResult(nodeName, 'Power Supply', systemResults.hardware?.powerSupply, `${nodeName}-hardware-power-supply`, true)}
                                                  {renderCheckResult(nodeName, 'Memory Errors', systemResults.hardware?.memoryErrors, `${nodeName}-hardware-memory-errors`, true)}
                                                  {renderCheckResult(nodeName, 'PCIe Errors', systemResults.hardware?.pcieErrors, `${nodeName}-hardware-pcie-errors`, true)}
                                                  {renderCheckResult(nodeName, 'CPU Microcode', systemResults.hardware?.cpuMicrocode, `${nodeName}-hardware-cpu-microcode`, true)}

                                                  <h3 style={{ fontSize: '1.2rem', marginTop: '1.5rem', marginBottom: '0.5rem' }}>Disks</h3>
                                                  {renderCheckResult(nodeName, 'Disk Space', systemResults.disks?.space, `${nodeName}-disk-space`, true)}
                                                  {renderCheckResult(nodeName, 'SMART', systemResults.disks?.smart, `${nodeName}-disk-smart`, true)}
                                                  {renderCheckResult(nodeName, 'Disk Performance', systemResults.disks?.performance, `${nodeName}-disk-performance`, true)}
                                                  {renderCheckResult(nodeName, 'RAID', systemResults.disks?.raid, `${nodeName}-disk-raid`, true)}
                                                  {renderCheckResult(nodeName, 'LVM Physical Volumes', systemResults.disks?.pvs, `${nodeName}-disk-pvs`, true)}
                                                  {renderCheckResult(nodeName, 'LVM Logical Volumes', systemResults.disks?.lvm, `${nodeName}-disk-lvm`, true)}
                                                  {renderCheckResult(nodeName, 'I/O Wait', systemResults.disks?.ioWait, `${nodeName}-disk-io-wait`, true)}
                                                  {renderCheckResult(nodeName, 'Queue Depth', systemResults.disks?.queueDepth, `${nodeName}-disk-queue-depth`, true)}
                                                  {renderCheckResult(nodeName, 'Filesystem Errors', systemResults.disks?.filesystemErrors, `${nodeName}-disk-filesystem-errors`, true)}
                                                  {renderCheckResult(nodeName, 'Inode Usage', systemResults.disks?.inodeUsage, `${nodeName}-disk-inode-usage`, true)}
                                                  {renderCheckResult(nodeName, 'Mount Points', systemResults.disks?.mountPoints, `${nodeName}-disk-mount-points`, true)}

                                                  <h3 style={{ fontSize: '1.2rem', marginTop: '1.5rem', marginBottom: '0.5rem' }}>Network</h3>
                                                  {renderCheckResult(nodeName, 'Interfaces', systemResults.network?.interfaces, `${nodeName}-network-interfaces`, true)}
                                                  {renderCheckResult(nodeName, 'Routing', systemResults.network?.routing, `${nodeName}-network-routing`, true)}
                                                  {renderCheckResult(nodeName, 'Connectivity', systemResults.network?.connectivity, `${nodeName}-network-connectivity`, true)}
                                                  {renderCheckResult(nodeName, 'Statistics', systemResults.network?.statistics, `${nodeName}-network-statistics`, true)}
                                                  {renderCheckResult(nodeName, 'Errors', systemResults.network?.errors, `${nodeName}-network-errors`, true)}
                                                  {renderCheckResult(nodeName, 'Latency', systemResults.network?.latency, `${nodeName}-network-latency`, true)}
                                                  {renderCheckResult(nodeName, 'DNS Resolution', systemResults.network?.dnsResolution, `${nodeName}-network-dns-resolution`, true)}
                                                  {renderCheckResult(nodeName, 'Bonding Status', systemResults.network?.bondingStatus, `${nodeName}-network-bonding-status`, true)}
                                                  {renderCheckResult(nodeName, 'Firewall Rules', systemResults.network?.firewallRules, `${nodeName}-network-firewall-rules`, true)}
                                                  
                                                  {!hasSystemResults && (
                                                    <Card style={{ marginTop: '1rem' }}>
                                                      <CardBody>
                                                        <p style={{ color: '#666', fontStyle: 'italic' }}>
                                                          Nessun check di sistema disponibile al momento.
                                                        </p>
                                                      </CardBody>
                                                    </Card>
                                                  )}
                                                </>
                                              ) : (
                                                <Card style={{ marginTop: '1rem' }}>
                                                  <CardBody>
                                                    <p style={{ color: '#666', fontStyle: 'italic' }}>
                                                      Dati dei check di sistema non disponibili.
                                                    </p>
                                                  </CardBody>
                                                </Card>
                                              )}
                                            </div>
                                          </Tab>
                                          
                                          <Tab eventKey={1} title="Kubernetes Checks">
                                            <div style={{ marginTop: '1rem' }}>
                                              {kubernetesResults ? (
                                                <>
                                                  {renderCheckResult(nodeName, 'Node Status', kubernetesResults.nodeStatus, `${nodeName}-k8s-node-status`, true)}
                                                  {renderCheckResult(nodeName, 'Pods', kubernetesResults.pods, `${nodeName}-k8s-pods`, true)}
                                                  {renderCheckResult(nodeName, 'Cluster Operators', kubernetesResults.clusterOperators, `${nodeName}-k8s-cluster-operators`, true)}
                                                  {renderCheckResult(nodeName, 'Node Resources', kubernetesResults.nodeResources, `${nodeName}-k8s-node-resources`, true)}
                                                  {renderCheckResult(nodeName, 'Node Resource Usage', kubernetesResults.nodeResourceUsage, `${nodeName}-k8s-node-resource-usage`, true)}
                                                  {renderCheckResult(nodeName, 'Container Runtime', kubernetesResults.containerRuntime, `${nodeName}-k8s-container-runtime`, true)}
                                                  {renderCheckResult(nodeName, 'Kubelet Health', kubernetesResults.kubeletHealth, `${nodeName}-k8s-kubelet-health`, true)}
                                                  {renderCheckResult(nodeName, 'CNI Plugin', kubernetesResults.cniPlugin, `${nodeName}-k8s-cni-plugin`, true)}
                                                  {renderCheckResult(nodeName, 'Node Conditions', kubernetesResults.nodeConditions, `${nodeName}-k8s-node-conditions`, true)}
                                                  
                                                  {!hasKubernetesResults && (
                                                    <Card style={{ marginTop: '1rem' }}>
                                                      <CardBody>
                                                        <p style={{ color: '#666', fontStyle: 'italic' }}>
                                                          Nessun check Kubernetes disponibile al momento.
                                                        </p>
                                                      </CardBody>
                                                    </Card>
                                                  )}
                                                </>
                                              ) : (
                                                <Card style={{ marginTop: '1rem' }}>
                                                  <CardBody>
                                                    <p style={{ color: '#666', fontStyle: 'italic' }}>
                                                      Dati dei check Kubernetes non disponibili.
                                                    </p>
                                                  </CardBody>
                                                </Card>
                                              )}
                                            </div>
                                          </Tab>
                                        </Tabs>
                                    </div>
                                  </div>
                                );
                              })()}
                </CardBody>
              </Card>
                        );
                      })
                    )}
                  </div>
                </Tab>
              </Tabs>
            </div>
          </section>
        </main>
      </div>
    </div>
  );
};

// Export sia come default che come named per compatibilità
export default NodeCheckOverview;
export { NodeCheckOverview };
