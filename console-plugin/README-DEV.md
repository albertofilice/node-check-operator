# Local Console Plugin Development

This guide explains how to test the console plugin locally before deploying it to OpenShift.

## Prerequisites

- Node.js 16+ and npm
- All dependencies installed: `npm install`

## Setup

### 1. Install Dependencies

```bash
cd console-plugin
npm install
```

### 2. Start Development Environment

There are two ways to start the development environment:

#### Option A: Mock API Server + Webpack Dev Server (recommended for isolated testing)

```bash
npm run dev:all
```

This command starts:
- Mock API Server on `http://localhost:3001` (simulates the operator backend)
- Webpack Dev Server on `http://localhost:9001` (serves the plugin)

#### Option B: Webpack Dev Server Only (if you already have a backend running)

```bash
npm run dev
```

And in another terminal, if needed:
```bash
npm run dev:mock
```

## Local Testing

### Interactive Test Page

Open the `test-local.html` file in your browser to run automatic tests:

```bash
# Open the file in browser
open test-local.html
# or
firefox test-local.html
# or
google-chrome test-local.html
```

The test page verifies:
- That services are running (Mock API and Webpack Dev Server)
- That mock APIs work correctly
- That the plugin manifest is generated and valid
- That plugin files are accessible

### Access Plugin Files

The webpack dev server serves plugin files at:
```
http://localhost:9001/api/plugins/node-check-console-plugin/
```

### Mock API Endpoints

The mock server provides the following endpoints:

- `GET /api/v1/stats` - General statistics
- `GET /api/v1/nodechecks` - List of all NodeChecks
- `GET /api/v1/nodechecks/:name` - Details of a specific NodeCheck
- `GET /api/v1/nodes/:nodeName` - Information about a node
- `GET /api/v1/nodes/:nodeName/pods` - Pods on a node

### Manual Verification

1. **Verify Mock API:**
   ```bash
   curl http://localhost:3001/api/v1/stats
   ```

2. **Verify Plugin Manifest:**
   ```bash
   curl http://localhost:9001/api/plugins/node-check-console-plugin/plugin-manifest.json
   ```

3. **Verify webpack compiles:**
   - Check the terminal where you started `npm run dev:all`
   - You should see "webpack compiled successfully"

### Debugging

With development mode, you will have:
- **Source Maps**: Detailed errors with references to original source code
- **Hot Module Replacement**: File changes are automatically reloaded
- **Console Logs**: All debug logs are visible in the browser console

### Check for Errors

1. Open the browser console (F12)
2. React errors will show complete stack traces instead of minified errors
3. Errors will include references to original source files thanks to source maps

## Testing with OpenShift Console (Optional)

To test the plugin loaded in the real OpenShift Console:

1. Build the plugin: `npm run build`
2. Deploy the plugin to OpenShift
3. Access the OpenShift console and navigate to the plugin page

## Troubleshooting

### Port Already in Use

If port 9001 or 3001 is already in use, modify the ports in:
- `webpack.config.js` (webpack dev server port)
- `mock-api-server.js` (mock API port)

### CORS Errors

The webpack dev server is configured with CORS headers. If you see CORS errors, verify that:
- CORS headers are configured correctly in `webpack.config.js`
- The mock API server is returning correct CORS headers

### Plugin Not Loading

Verify that:
- The plugin manifest is generated correctly in `dist/plugin-manifest.json`
- Files are served at the correct path: `/api/plugins/node-check-console-plugin/`
