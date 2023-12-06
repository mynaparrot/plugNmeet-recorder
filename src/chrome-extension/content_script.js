/* global chrome */

window.onload = () => {
  const port = chrome.runtime.connect(chrome.runtime.id);
  // listen for messages & post
  chrome.runtime.onMessage.addListener((msg) => window.postMessage(msg, '*'));

  let hasWebsocketError = false;
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
};
