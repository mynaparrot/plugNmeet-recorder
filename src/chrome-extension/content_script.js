/* global chrome */

window.onload = () => {
  const port = chrome.runtime.connect(chrome.runtime.id);
  // listen for messages & post
  chrome.runtime.onMessage.addListener((msg) => window.postMessage(msg, '*'));

  let senErrorMsg = false;
  window.addEventListener('message', (event) => {
    // Relay client messages
    if (event.source === window && event.data.type) {
      port.postMessage(event.data);
    }
    if (event.data.websocketError) {
      if (!senErrorMsg) {
        window.postMessage({
          type: 'WEBSOCKET_ERROR',
          msg: event.data.websocketError,
        });
      }
      senErrorMsg = true;
    } else if (event.data.tabCaptureError) {
      if (!senErrorMsg) {
        window.postMessage({
          type: 'TAB_CAPTURE_ERROR',
          msg: event.data.tabCaptureError,
        });
      }
      senErrorMsg = true;
    }
  });

  document.title = 'recorder';
  window.postMessage(
    { type: 'REC_CLIENT_PLAY', data: { url: window.location.origin } },
    '*',
  );
};
