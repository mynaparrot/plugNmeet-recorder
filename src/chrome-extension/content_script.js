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
    console.log(event.data);
    if (event.data.websocketError) {
      if (!hasWebsocketError) {
        window.postMessage({
          type: 'WEBSOCKET_ERROR',
          msg: event.data.websocketError,
        });
      }
      hasWebsocketError = true;
    } else if (event.data.tabCaptureError) {
      window.postMessage({
        type: 'TAB_CAPTURE_ERROR',
        msg: event.data.tabCaptureError,
      });
    }
  });

  document.title = 'recorder';
  window.postMessage(
    { type: 'REC_CLIENT_PLAY', data: { url: window.location.origin } },
    '*',
  );
};
