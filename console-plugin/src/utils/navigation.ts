/**
 * Utility per la navigazione client-side senza refresh della pagina
 * Usa window.history.pushState() per cambiare l'URL senza ricaricare la pagina
 * OpenShift Console usa React Router che dovrebbe reagire automaticamente ai cambiamenti di URL
 */

/**
 * Naviga a un nuovo URL senza ricaricare la pagina
 * @param url - L'URL di destinazione (es: "/nodecheck/name" o "/node/nodeName")
 */
export function navigateTo(url: string): void {
  // Verifica se l'URL è già quello corrente
  if (window.location.pathname + window.location.search === url) {
    console.log('Already at target URL:', url);
    return;
  }
  
  // Usa pushState per cambiare l'URL senza ricaricare la pagina
  // Questo aggiorna l'URL nella barra degli indirizzi senza ricaricare la pagina
  window.history.pushState({ path: url }, '', url);
  
  // OpenShift Console usa React Router che dovrebbe reagire automaticamente ai cambiamenti di URL
  // Tuttavia, per assicurarci che il router reagisca, possiamo dispatchare un evento popstate
  // Nota: pushState non triggera automaticamente un evento popstate, quindi lo facciamo manualmente
  const popStateEvent = new PopStateEvent('popstate', {
    state: { path: url },
  });
  window.dispatchEvent(popStateEvent);
  
  // Forza anche un evento hashchange come fallback (alcune implementazioni di routing reagiscono a questo)
  // Anche se non stiamo usando hash, questo può aiutare alcuni router a reagire
  window.dispatchEvent(new HashChangeEvent('hashchange'));
  
  console.log('Navigated to (no refresh):', url);
}

/**
 * Naviga indietro nella cronologia
 */
export function navigateBack(): void {
  window.history.back();
}

/**
 * Naviga avanti nella cronologia
 */
export function navigateForward(): void {
  window.history.forward();
}

