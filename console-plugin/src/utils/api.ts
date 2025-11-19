/**
 * Utility functions for API calls in OpenShift Console Plugin
 */

const PLUGIN_NAME = 'node-check-console-plugin';
const PROXY_ALIAS = 'api-v1';

/**
 * Costruisce l'URL per le richieste API attraverso il proxy di OpenShift Console
 * 
 * Formato proxy: /api/proxy/plugin/<plugin-name>/<proxy-alias>/<request-path>
 * 
 * Il proxy di OpenShift Console inoltra tutto il path dopo l'alias al backend.
 * Quindi se l'URL è: /api/proxy/plugin/node-check-console-plugin/api-v1/api/v1/stats
 * Il proxy inoltra al backend: /api/v1/stats
 * 
 * Per questo motivo, dobbiamo includere "api/v1/" nel path quando costruiamo l'URL.
 */
export function getProxyURL(path: string): string {
  // Rimuovi lo slash iniziale se presente
  const cleanPath = path.startsWith('/') ? path.slice(1) : path;
  
  // Se il path non inizia già con "api/v1/", aggiungilo
  // Questo è necessario perché il proxy inoltra tutto il path dopo l'alias
  const fullPath = cleanPath.startsWith('api/v1/') ? cleanPath : `api/v1/${cleanPath}`;
  
  // Costruisci l'URL completo per il proxy
  // Esempio: /api/proxy/plugin/node-check-console-plugin/api-v1/api/v1/stats
  return `/api/proxy/plugin/${PLUGIN_NAME}/${PROXY_ALIAS}/${fullPath}`;
}

/**
 * Esegue una richiesta GET all'API attraverso il proxy
 * @param path - Il path dell'endpoint API (es: "nodechecks/name" o "stats")
 * @param params - Parametri query opzionali (es: { namespace: "default" })
 */
export async function apiGet<T = any>(path: string, params?: Record<string, string>): Promise<T> {
  let url = getProxyURL(path);
  
  // Aggiungi parametri query se presenti
  if (params && Object.keys(params).length > 0) {
    const queryString = new URLSearchParams(params).toString();
    url += `?${queryString}`;
  }
  
  try {
    const response = await fetch(url);
    
    if (!response.ok) {
      // Prova a leggere il body per avere più dettagli sull'errore
      let errorMessage = `API request failed: ${response.status} ${response.statusText}`;
      try {
        const errorBody = await response.text();
        if (errorBody) {
          errorMessage += ` - ${errorBody}`;
        }
      } catch (e) {
        // Ignora errori nel parsing del body
      }
      
      throw new Error(errorMessage);
    }
    
    return response.json();
  } catch (error) {
    // Se è un errore di rete (fetch fallisce), fornisci più dettagli
    if (error instanceof TypeError && error.message.includes('fetch')) {
      throw new Error(`Network error: Unable to reach API at ${url}. Check if the proxy is configured correctly.`);
    }
    throw error;
  }
}

