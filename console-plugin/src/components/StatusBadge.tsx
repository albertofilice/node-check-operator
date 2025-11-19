import React from 'react';
import { Badge } from '@patternfly/react-core';

interface StatusBadgeProps {
  status: 'Healthy' | 'Warning' | 'Critical' | 'Unknown';
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status }) => {
  console.log('[StatusBadge] Rendering with status:', status);
  // Ensure status is never undefined
  const safeStatus = status || 'Unknown';
  
  const getVariant = (status: string) => {
    switch (status) {
      case 'Healthy':
        return 'success';
      case 'Warning':
        return 'warning';
      case 'Critical':
        return 'danger';
      default:
        return 'secondary';
    }
  };

  return (
    <Badge variant={getVariant(safeStatus)}>
      {safeStatus}
    </Badge>
  );
};
