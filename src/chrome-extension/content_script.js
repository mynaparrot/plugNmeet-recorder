/* global chrome, window, document */

window.onload = () => {
  if (window.recorderInjected) return;
  Object.defineProperty(window, 'recorderInjected', {
    value: true,
    writable: false,
  });

  // Setup message passing
  const port = chrome.runtime.connect(chrome.runtime.id);
  let hasWebsocketError = false;
  port.onMessage.addListener((msg) => window.postMessage(msg, '*'));
  window.addEventListener('message', (event) => {
    // Relay client messages
    if (event.source === window && event.data.type) {
      port.postMessage(event.data);
    }
    if (event.data.websocketError) {
      if (!hasWebsocketError) {
        window.postMessage({
          type: 'WEBSOCKET_ERROR',
        });
      }
      hasWebsocketError = true;
    }
  });

  document.title = 'recorder';
  window.postMessage(
    { type: 'REC_CLIENT_PLAY', data: { url: window.location.origin } },
    '*',
  );
};
