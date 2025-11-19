# Plugin Pages Structure

This document explains where charts and various sections of the plugin are displayed in the OpenShift Console.

## Main Page: Node Check Overview

**URL:** `/nodecheck`  
**Access:** Menu "Monitoring" → "Node Check" (link in sidebar)

### What you see on this page:

1. **Title:** "Node Check Overview"

2. **Statistics Section (Cards):**
   - 4 cards with numbers:
     - Healthy Nodes (green)
     - Warning Nodes (orange)
     - Critical Nodes (red)
     - Total Nodes (blue)

3. **Charts Section:**
   - **Donut Chart** - on the left
     - Shows percentage distribution: Healthy/Warning/Critical
     - Title: "Status Distribution"
   
   - **Bar Chart** - on the right
     - Visual comparison of statistics
     - Title: "Node Statistics"

4. **NodeChecks Table:**
   - List of all NodeChecks with:
     - Name
     - Node
     - Status (colored badge)
     - Last check
     - Message
     - "View Details" button

---

## NodeCheck Detail Page

**URL:** `/nodecheck/:name` (e.g. `/nodecheck/nodecheck-worker-1`)  
**Access:** Clicking "View Details" in the Overview page table

### What you see on this page:

1. **Basic Information:**
   - Name, Namespace, Node, Status

2. **Check Statistics:**
   - Last Check, Total Checks, Healthy, Warnings, Critical

3. **System Checks Tab:**
   - Uptime
   - Processes
   - Resources
   - Services
   - Memory
   - System Logs
   - Hardware (Temperature, IPMI, BMC)
   - Disks (Space, SMART, Performance, RAID, PVs, LVM)
   - Network (Interfaces, Routing, Connectivity, Statistics)

4. **Kubernetes Checks Tab:**
   - Node Status
   - Pods
   - Cluster Operators
   - Node Resources

**Note:** This page does NOT have charts (only textual details).

---

## Node Detail Page

**URL:** `/node/:nodeName` (e.g. `/node/worker-1`)  
**Access:** Clicking the node name in the Overview page table

### What you see on this page:

1. **Node Information:**
   - Name, Creation Time, Unschedulable
   - Conditions
   - Capacity and Allocatable

2. **Pods on Node:**
   - Table with all pods running on the node

3. **Node NodeChecks:**
   - List of NodeChecks associated with this node

**Note:** This page does NOT have charts (only textual information).

---

## Summary: Where are the Charts?

**CHARTS PRESENT:**
- **Only on the Overview page** (`/nodecheck`)
  - Donut Chart (Status Distribution)
  - Bar Chart (Node Statistics)

**CHARTS NOT PRESENT:**
- NodeCheck Detail Page (`/nodecheck/:name`)
- Node Detail Page (`/node/:nodeName`)

---

## How to Access the Plugin

1. **From OpenShift Console:**
   - Go to the side menu
   - Click on "Monitoring"
   - Click on "Node Check"
   - See the Overview page with charts!

2. **From Kubernetes Resources List:**
   - Go to "Resources" → "NodeCheck"
   - See the NodeCheck list (uses the same Overview page)

3. **Direct URL:**
   - `https://console-openshift-console.apps.<cluster>/nodecheck`

---

## Notes

- Charts are displayed only if there are NodeChecks (`totalNodeChecks > 0`)
- Charts update automatically every 30 seconds
- Chart colors correspond to card colors (green/orange/red)
